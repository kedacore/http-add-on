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
	"time"

	"github.com/kedacore/http-add-on/pkg/env"
	"github.com/kedacore/http-add-on/pkg/k8s"
	externalscaler "github.com/kedacore/http-add-on/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	// The port to run this scaler server on
	portStr, err := env.Get("KEDA_HTTP_SCALER_PORT")
	if err != nil {
		log.Fatalf("KEDA_HTTP_SCALER_PORT not found")
	}
	// The namespace that the interceptors service is in
	namespace, err := env.Get("KEDA_HTTP_SCALER_TARGET_ADMIN_NAMESPACE")
	if err != nil {
		log.Fatalf("KEDA_HTTP_SCALER_TARGET_ADMIN_NAMESPACE not found")
	}
	// The name of the interceptor service that has the queue size ("/queue") endpoint on
	svcName, err := env.Get("KEDA_HTTP_SCALER_TARGET_ADMIN_SERVICE")
	if err != nil {
		log.Fatalf("KEDA_HTTP_SCALER_TARGET_ADMIN_SERVICE not found")
	}
	// The port that the interceptor service is on
	targetPortStr, err := env.Get("KEDA_HTTP_SCALER_TARGET_ADMIN_PORT")
	if err != nil {
		log.Fatalf("KEDA_HTTP_SCALER_TARGET_ADMIN_PORT not found")
	}
	k8sCl, _, err := k8s.NewClientset()
	if err != nil {
		log.Fatalf("Couldn't get a Kubernetes client (%s)", err)
	}
	pinger := newQueuePinger(
		context.Background(),
		k8sCl,
		namespace,
		svcName,
		targetPortStr,
		time.NewTicker(500*time.Millisecond),
	)
	log.Fatal(startGrpcServer(portStr, pinger))
}

func startGrpcServer(port string, pinger *queuePinger) error {
	addr := fmt.Sprintf("0.0.0.0:%s", port)
	log.Printf("Serving external scaler on %s", addr)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	grpcServer := grpc.NewServer()
	externalscaler.RegisterExternalScalerServer(grpcServer, newImpl(pinger))
	reflection.Register(grpcServer)
	return grpcServer.Serve(lis)
}
