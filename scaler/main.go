// The HTTP Scaler is the standard implementation for a KEDA external scaler
// which can be found at https://keda.sh/docs/2.0/concepts/external-scalers/
// This scaler has the implementation of an HTTP request counter and informs
// KEDA of the current request number for the queue in order to scale the app
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	_ "net/http/pprof" //nolint:gosec // G108: pprof intentionally exposed, gated by --profiling-addr
	"os"
	"time"

	"github.com/go-logr/logr"
	"github.com/kedacore/keda/v2/pkg/scalers/externalscaler"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/kedacore/http-add-on/pkg/build"
	kedacache "github.com/kedacore/http-add-on/pkg/cache"
	"github.com/kedacore/http-add-on/pkg/k8s"
	"github.com/kedacore/http-add-on/pkg/util"
)

var setupLog = ctrl.Log.WithName("setup")

// +kubebuilder:rbac:groups=discovery.k8s.io,resources=endpointslices,verbs=get;list;watch
// +kubebuilder:rbac:groups=http.keda.sh,resources=httpscaledobjects,verbs=get;list;watch

func main() {
	cfg := mustParseConfig()
	namespace := cfg.TargetNamespace
	svcName := cfg.TargetService
	deplName := cfg.TargetDeployment
	targetPortStr := fmt.Sprintf("%d", cfg.TargetPort)
	profilingAddr := cfg.ProfilingAddr

	opts := zap.Options{}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	k8sCfg, err := ctrl.GetConfig()
	if err != nil {
		setupLog.Error(err, "Kubernetes client config not found")
		os.Exit(1)
	}

	ctrlCache, err := cache.New(k8sCfg, cache.Options{
		Scheme:     kedacache.NewScheme(),
		SyncPeriod: &cfg.CacheSyncPeriod,
	})
	if err != nil {
		setupLog.Error(err, "creating cache")
		os.Exit(1)
	}

	endpointsCache, err := k8s.NewInformerBackedEndpointsCache(ctrl.Log, ctrlCache)
	if err != nil {
		setupLog.Error(err, "creating endpoints cache")
		os.Exit(1)
	}

	pinger := newQueuePinger(ctrl.Log, k8s.EndpointsFuncForControllerClient(ctrlCache), namespace, svcName, deplName, targetPortStr)

	ctx := ctrl.SetupSignalHandler()
	ctx = util.ContextWithLogger(ctx, setupLog)

	eg, ctx := errgroup.WithContext(ctx)

	// start the controller-runtime cache
	eg.Go(func() error {
		setupLog.Info("starting the controller-runtime cache")
		return ctrlCache.Start(ctx)
	})

	eg.Go(func() error {
		setupLog.Info("starting the queue pinger")

		if err := pinger.start(ctx, time.NewTicker(cfg.QueueTickDuration), endpointsCache); !util.IsIgnoredErr(err) {
			setupLog.Error(err, "queue pinger failed")
			return err
		}

		return nil
	})

	if len(profilingAddr) > 0 {
		eg.Go(func() error {
			setupLog.Info("enabling pprof for profiling", "address", profilingAddr)
			srv := &http.Server{
				Addr:              profilingAddr,
				ReadHeaderTimeout: time.Minute, // mitigate Slowloris attacks
			}
			return srv.ListenAndServe()
		})
	}

	// Wait for cache to sync before starting the GRPC server
	if !ctrlCache.WaitForCacheSync(ctx) {
		setupLog.Error(nil, "cache failed to sync")
		os.Exit(1)
	}

	eg.Go(func() error {
		setupLog.Info("starting the grpc server")

		if err := startGrpcServer(ctx, cfg, ctrl.Log, pinger, ctrlCache); !util.IsIgnoredErr(err) {
			setupLog.Error(err, "grpc server failed")
			return err
		}

		return nil
	})

	build.PrintComponentInfo(ctrl.Log, "Scaler")

	if err := eg.Wait(); err != nil && !errors.Is(err, context.Canceled) {
		setupLog.Error(err, "fatal error")
		os.Exit(1)
	}

	setupLog.Info("Bye!")
}

func startGrpcServer(ctx context.Context, cfg config, lggr logr.Logger, pinger *queuePinger, reader client.Reader) error {
	addr := fmt.Sprintf("0.0.0.0:%d", cfg.GRPCPort)
	lggr.Info("starting grpc server", "address", addr)

	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	grpcServer := grpc.NewServer()
	reflection.Register(grpcServer)

	hs := health.NewServer()
	go func() {
		lggr.Info("starting healthchecks loop")
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			// handle cancellations/timeout
			case <-ctx.Done():
				hs.SetServingStatus("liveness", grpc_health_v1.HealthCheckResponse_NOT_SERVING)
				hs.SetServingStatus("readiness", grpc_health_v1.HealthCheckResponse_NOT_SERVING)
				return
			// do our regularly scheduled work
			case <-ticker.C:
				// if we haven't updated the endpoints in twice QueueTickDuration we drop the check
				if time.Now().After(pinger.lastPingTime.Add(cfg.QueueTickDuration * 2)) {
					hs.SetServingStatus("liveness", grpc_health_v1.HealthCheckResponse_NOT_SERVING)
					hs.SetServingStatus("readiness", grpc_health_v1.HealthCheckResponse_NOT_SERVING)
				} else {
					// we propagate pinger status as scaler status
					hs.SetServingStatus("liveness", grpc_health_v1.HealthCheckResponse_ServingStatus(pinger.status))
					hs.SetServingStatus("readiness", grpc_health_v1.HealthCheckResponse_ServingStatus(pinger.status))
				}
			}
		}
	}()

	grpc_health_v1.RegisterHealthServer(grpcServer, hs)

	externalscaler.RegisterExternalScalerServer(grpcServer, newScalerHandler(lggr, pinger, reader, time.Duration(cfg.StreamIntervalMS)*time.Millisecond))

	go func() {
		<-ctx.Done()
		grpcServer.GracefulStop()
	}()

	return grpcServer.Serve(lis)
}
