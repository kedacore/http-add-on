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
		log.Fatalf("Couldn't get a Kubernetes client (%s)", err)
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
	grp.Go(startHealthcheckServer(ctx, healthPort))
	log.Fatalf("One or more of the servers failed: %s", grp.Wait())
}

func startGrpcServer(
	ctx context.Context,
	lggr logr.Logger,
	port int,
	pinger *queuePinger,
) func() error {
	return func() error {
		addr := fmt.Sprintf("0.0.0.0:%d", port)
		log.Printf("Serving external scaler on %s", addr)
		lis, err := net.Listen("tcp", addr)
		if err != nil {
			log.Fatalf("failed to listen: %v", err)
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

func startHealthcheckServer(ctx context.Context, port int) func() error {
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
		log.Printf("Serving health check server on port %d", port)

		go func() {
			<-ctx.Done()
			srv.Shutdown(ctx)
		}()
		return srv.ListenAndServe()
	}
}
