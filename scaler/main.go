// The HTTP Scaler is the standard implementation for a KEDA external scaler
// which can be found at https://keda.sh/docs/2.0/concepts/external-scalers/
// This scaler has the implementation of an HTTP request counter and informs
// KEDA of the current request number for the queue in order to scale the app
package main

import (
	"context"
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
	k8sCl, _, err := k8s.NewClientset()
	if err != nil {
		lggr.Error(err, "getting a Kubernetes client")
		os.Exit(1)
	}
	pinger := newQueuePinger(
		context.Background(),
		lggr,
		k8sCl,
		namespace,
		svcName,
		targetPortStr,
		time.NewTicker(500*time.Millisecond),
	)

	grp, ctx := errgroup.WithContext(ctx)
	grp.Go(startGrpcServer(ctx, lggr, grpcPort, pinger))
	grp.Go(startHealthcheckServer(ctx, lggr, healthPort))
	lggr.Error(grp.Wait(), "one or more of the servers failed")
}

func startGrpcServer(
	ctx context.Context,
	lggr logr.Logger,
	port int,
	pinger *queuePinger,
) func() error {
	return func() error {
		addr := fmt.Sprintf("0.0.0.0:%d", port)
		lggr.Info("starting grpc server", "address", addr)
		lis, err := net.Listen("tcp", addr)
		if err != nil {
			return err
		}

		grpcServer := grpc.NewServer()
		externalscaler.RegisterExternalScalerServer(grpcServer, newImpl(pinger))
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
) func() error {
	return func() error {
		mux := http.NewServeMux()
		mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(200)
		})
		mux.HandleFunc("/livez", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(200)
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
