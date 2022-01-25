// The HTTP Scaler is the standard implementation for a KEDA external scaler
// which can be found at https://keda.sh/docs/2.0/concepts/external-scalers/
// This scaler has the implementation of an HTTP request counter and informs
// KEDA of the current request number for the queue in order to scale the app
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/go-logr/logr"
	kedahttp "github.com/kedacore/http-add-on/pkg/http"
	"github.com/kedacore/http-add-on/pkg/k8s"
	pkglog "github.com/kedacore/http-add-on/pkg/log"
	"github.com/kedacore/http-add-on/pkg/routing"
	externalscaler "github.com/kedacore/http-add-on/proto"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	lggr, err := pkglog.NewZapr()
	if err != nil {
		log.Fatalf("error creating new logger (%v)", err)
	}
	ctx, done := context.WithCancel(
		context.Background(),
	)
	defer done()
	cfg := mustParseConfig()
	grpcPort := cfg.GRPCPort
	healthPort := cfg.HealthPort
	namespace := cfg.TargetNamespace
	svcName := cfg.TargetService
	targetPortStr := fmt.Sprintf("%d", cfg.TargetPort)
	targetPendingRequests := cfg.TargetPendingRequests
	targetPendingRequestsInterceptor := cfg.TargetPendingRequestsInterceptor

	k8sCl, _, err := k8s.NewClientset()
	if err != nil {
		lggr.Error(err, "getting a Kubernetes client")
		os.Exit(1)
	}
	pinger, err := newQueuePinger(
		context.Background(),
		lggr,
		k8s.EndpointsFuncForK8sClientset(k8sCl),
		namespace,
		svcName,
		targetPortStr,
	)
	if err != nil {
		lggr.Error(err, "creating a queue pinger")
		os.Exit(1)
	}

	// This callback function is used to fetch and save
	// the current queue counts from the interceptor immediately
	// after updating the routingTable information.
	callbackWhenRoutingTableUpdate := func() error {
		if err := pinger.fetchAndSaveCounts(ctx); err != nil {
			return err
		}
		return nil
	}

	table := routing.NewTable()

	// Create the informer of ConfigMap resource,
	// the resynchronization period of the informer should be not less than 1s,
	// refer to: https://github.com/kubernetes/client-go/blob/v0.22.2/tools/cache/shared_informer.go#L475
	configMapInformer := k8s.NewInformerConfigMapUpdater(
		lggr,
		k8sCl,
		cfg.ConfigMapCacheRsyncPeriod,
	)

	grp, ctx := errgroup.WithContext(ctx)

	grp.Go(func() error {
		defer done()
		return pinger.start(
			ctx,
			time.NewTicker(cfg.QueueTickDuration),
		)
	})

	grp.Go(func() error {
		defer done()
		return startGrpcServer(
			ctx,
			lggr,
			grpcPort,
			pinger,
			table,
			int64(targetPendingRequests),
			int64(targetPendingRequestsInterceptor),
		)
	})

	grp.Go(func() error {
		defer done()
		return routing.StartConfigMapRoutingTableUpdater(
			ctx,
			lggr,
			configMapInformer,
			cfg.TargetNamespace,
			table,
			callbackWhenRoutingTableUpdate,
		)
	})

	grp.Go(func() error {
		defer done()
		return startAdminServer(
			ctx,
			lggr,
			cfg,
			healthPort,
			pinger,
		)
	})
	lggr.Error(grp.Wait(), "one or more of the servers failed")
}

func startGrpcServer(
	ctx context.Context,
	lggr logr.Logger,
	port int,
	pinger *queuePinger,
	routingTable *routing.Table,
	targetPendingRequests int64,
	targetPendingRequestsInterceptor int64,
) error {

	addr := fmt.Sprintf("0.0.0.0:%d", port)
	lggr.Info("starting grpc server", "address", addr)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	grpcServer := grpc.NewServer()
	externalscaler.RegisterExternalScalerServer(
		grpcServer,
		newImpl(
			lggr,
			pinger,
			routingTable,
			targetPendingRequests,
			targetPendingRequestsInterceptor,
		),
	)
	reflection.Register(grpcServer)
	go func() {
		<-ctx.Done()
		lis.Close()
	}()
	return grpcServer.Serve(lis)
}

func startAdminServer(
	ctx context.Context,
	lggr logr.Logger,
	cfg *config,
	port int,
	pinger *queuePinger,
) error {
	lggr = lggr.WithName("startHealthcheckServer")

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})
	mux.HandleFunc("/livez", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})
	mux.HandleFunc("/queue", func(w http.ResponseWriter, r *http.Request) {
		lggr = lggr.WithName("route.counts")
		cts := pinger.counts()
		lggr.Info("counts endpoint", "counts", cts)
		if err := json.NewEncoder(w).Encode(&cts); err != nil {
			lggr.Error(err, "writing counts information to client")
			w.WriteHeader(500)
		}
	})
	mux.HandleFunc("/queue_ping", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		lggr := lggr.WithName("route.counts_ping")
		if err := pinger.fetchAndSaveCounts(ctx); err != nil {
			lggr.Error(err, "requesting counts failed")
			w.WriteHeader(500)
			w.Write([]byte("error requesting counts from interceptors"))
			return
		}
		cts := pinger.counts()
		lggr.Info("counts ping endpoint", "counts", cts)
		if err := json.NewEncoder(w).Encode(&cts); err != nil {
			lggr.Error(err, "writing counts data to caller")
			w.WriteHeader(500)
			w.Write([]byte("error writing counts data to caller"))
		}
	})

	kedahttp.AddConfigEndpoint(lggr, mux, cfg)
	kedahttp.AddVersionEndpoint(lggr.WithName("scalerAdmin"), mux)

	addr := fmt.Sprintf("0.0.0.0:%d", port)
	lggr.Info("starting health check server", "addr", addr)
	return kedahttp.ServeContext(ctx, addr, mux)
}
