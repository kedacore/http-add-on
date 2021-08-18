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
	"github.com/kedacore/http-add-on/pkg/k8s"
	pkglog "github.com/kedacore/http-add-on/pkg/log"
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
	ctx := context.Background()
	cfg := mustParseConfig()
	grpcPort := cfg.GRPCPort
	healthPort := cfg.HealthPort
	namespace := cfg.TargetNamespace
	svcName := cfg.TargetService
	targetPortStr := fmt.Sprintf("%d", cfg.TargetPort)
	targetPendingRequests := cfg.TargetPendingRequests

	k8sCl, _, err := k8s.NewClientset()
	if err != nil {
		lggr.Error(err, "getting a Kubernetes client")
		os.Exit(1)
	}
	pinger := newQueuePinger(
		context.Background(),
		lggr,
		k8s.EndpointsFuncForK8sClientset(k8sCl),
		namespace,
		svcName,
		targetPortStr,
		time.NewTicker(500*time.Millisecond),
	)

	grp, ctx := errgroup.WithContext(ctx)
	grp.Go(
		startGrpcServer(
			ctx,
			lggr,
			grpcPort,
			pinger,
			int64(targetPendingRequests),
		),
	)
	grp.Go(startHealthcheckServer(ctx, lggr, healthPort, pinger))
	lggr.Error(grp.Wait(), "one or more of the servers failed")
}

func startGrpcServer(
	ctx context.Context,
	lggr logr.Logger,
	port int,
	pinger *queuePinger,
	targetPendingRequests int64,
) func() error {
	return func() error {
		addr := fmt.Sprintf("0.0.0.0:%d", port)
		lggr.Info("starting grpc server", "address", addr)
		lis, err := net.Listen("tcp", addr)
		if err != nil {
			return err
		}

		grpcServer := grpc.NewServer()
		externalscaler.RegisterExternalScalerServer(
			grpcServer,
			newImpl(lggr, pinger, targetPendingRequests),
		)
		reflection.Register(grpcServer)
		go func() {
			<-ctx.Done()
			lis.Close()
		}()
		return grpcServer.Serve(lis)
	}
}

func startHealthcheckServer(
	ctx context.Context,
	lggr logr.Logger,
	port int,
	pinger *queuePinger,
) func() error {
	lggr = lggr.WithName("startHealthcheckServer")
	return func() error {
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
			if err := pinger.requestCounts(ctx); err != nil {
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

		srv := &http.Server{
			Addr:    fmt.Sprintf(":%d", port),
			Handler: mux,
		}
		lggr.Info("starting health check server", "port", port)

		go func() {
			<-ctx.Done()
			srv.Shutdown(ctx)
		}()
		return srv.ListenAndServe()
	}
}
