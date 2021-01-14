// The HTTP Scaler is the standard implementation for a KEDA external scaler
// which can be found at https://keda.sh/docs/2.0/concepts/external-scalers/
// This scaler has the implementation of an HTTP request counter and informs
// KEDA of the current request number for the queue in order to scale the app
package main

import (
	"fmt"
	"log"
	"net"
	"time"

	"github.com/kedacore/http-add-on/pkg/env"
	"github.com/kedacore/http-add-on/pkg/k8s"
	externalscaler "github.com/kedacore/http-add-on/scaler/gen/scaler"
	"google.golang.org/grpc"
)

func main() {
	portStr := env.GetOr("8080", "KEDA_HTTP_SCALER_PORT")
	namespace, err := env.Get("KEDA_HTTP_SCALER_TARGET_ADMIN_NAMESPACE")
	if err != nil {
		log.Fatalf("KEDA_HTTP_SCALER_TARGET_ADMIN_NAMESPACE not found")
	}
	svcName, err := env.Get("KEDA_HTTP_SCALER_TARGET_ADMIN_SERVICE")
	if err != nil {
		log.Fatalf("KEDA_HTTP_SCALER_TARGET_ADMIN_SERVICE not found")
	}
	targetPortStr := env.GetOr("8081", "KEDA_HTTP_SCALER_TARGET_ADMIN_PORT")
	k8sCl, _, err := k8s.NewClientset()
	if err != nil {
		log.Fatalf("Couldn't get a Kubernetes client (%s)", err)
	}
	pinger := newQueuePinger(
		k8sCl,
		namespace,
		svcName,
		targetPortStr,
		time.NewTicker(500*time.Millisecond),
	)
	log.Fatal(startGrpcServer(portStr, pinger))
}

func startGrpcServer(port string, pinger queuePinger) error {
	lis, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%s", port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	grpcServer := grpc.NewServer()
	externalscaler.RegisterExternalScalerServer(grpcServer, newImpl(pinger))
	return grpcServer.Serve(lis)
}
