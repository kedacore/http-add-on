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
	"os"
	"time"

	"github.com/go-logr/logr"
	"github.com/kedacore/keda/v2/pkg/scalers/externalscaler"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	clientset "github.com/kedacore/http-add-on/operator/generated/clientset/versioned"
	informers "github.com/kedacore/http-add-on/operator/generated/informers/externalversions"
	informershttpv1alpha1 "github.com/kedacore/http-add-on/operator/generated/informers/externalversions/http/v1alpha1"
	"github.com/kedacore/http-add-on/pkg/build"
	"github.com/kedacore/http-add-on/pkg/k8s"
	"github.com/kedacore/http-add-on/pkg/util"
)

var (
	setupLog = ctrl.Log.WithName("setup")
)

// +kubebuilder:rbac:groups="",resources=endpoints,verbs=get;list;watch
// +kubebuilder:rbac:groups=http.keda.sh,resources=httpscaledobjects,verbs=get;list;watch

func main() {
	cfg := mustParseConfig()
	grpcPort := cfg.GRPCPort
	namespace := cfg.TargetNamespace
	svcName := cfg.TargetService
	deplName := cfg.TargetDeployment
	targetPortStr := fmt.Sprintf("%d", cfg.TargetPort)
	targetPendingRequests := cfg.TargetPendingRequests

	opts := zap.Options{}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	k8sCfg, err := ctrl.GetConfig()
	if err != nil {
		setupLog.Error(err, "Kubernetes client config not found")
		os.Exit(1)
	}
	k8sCl, err := kubernetes.NewForConfig(k8sCfg)
	if err != nil {
		setupLog.Error(err, "creating new Kubernetes ClientSet")
		os.Exit(1)
	}
	pinger := newQueuePinger(
		ctrl.Log,
		k8s.EndpointsFuncForK8sClientset(k8sCl),
		namespace,
		svcName,
		deplName,
		targetPortStr,
	)

	// create the endpoints informer
	endpInformer := k8s.NewInformerBackedEndpointsCache(
		ctrl.Log,
		k8sCl,
		cfg.DeploymentCacheRsyncPeriod,
	)

	httpCl, err := clientset.NewForConfig(k8sCfg)
	if err != nil {
		setupLog.Error(err, "creating new HTTP ClientSet")
		os.Exit(1)
	}
	sharedInformerFactory := informers.NewSharedInformerFactory(httpCl, cfg.ConfigMapCacheRsyncPeriod)
	httpsoInformer := informershttpv1alpha1.New(sharedInformerFactory, "", nil).HTTPScaledObjects()

	ctx := ctrl.SetupSignalHandler()
	ctx = util.ContextWithLogger(ctx, setupLog)

	eg, ctx := errgroup.WithContext(ctx)

	// start the endpoints informer
	eg.Go(func() error {
		setupLog.Info("starting the endpoints informer")

		endpInformer.Start(ctx)
		return nil
	})

	// start the httpso informer
	eg.Go(func() error {
		setupLog.Info("starting the httpso informer")

		httpsoInformer.Informer().Run(ctx.Done())
		return nil
	})

	eg.Go(func() error {
		setupLog.Info("starting the queue pinger")

		if err := pinger.start(ctx, time.NewTicker(cfg.QueueTickDuration), endpInformer); !util.IsIgnoredErr(err) {
			setupLog.Error(err, "queue pinger failed")
			return err
		}

		return nil
	})

	eg.Go(func() error {
		setupLog.Info("starting the grpc server")

		if err := startGrpcServer(ctx, cfg, ctrl.Log, grpcPort, pinger, httpsoInformer, int64(targetPendingRequests)); !util.IsIgnoredErr(err) {
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

func startGrpcServer(
	ctx context.Context,
	cfg *config,
	lggr logr.Logger,
	port int,
	pinger *queuePinger,
	httpsoInformer informershttpv1alpha1.HTTPScaledObjectInformer,
	targetPendingRequests int64,
) error {
	addr := fmt.Sprintf("0.0.0.0:%d", port)
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

	grpc_health_v1.RegisterHealthServer(
		grpcServer,
		hs,
	)

	externalscaler.RegisterExternalScalerServer(
		grpcServer,
		newImpl(
			lggr,
			pinger,
			httpsoInformer,
			targetPendingRequests,
		),
	)

	go func() {
		<-ctx.Done()
		grpcServer.GracefulStop()
	}()

	return grpcServer.Serve(lis)
}
