// The HTTP Scaler is the standard implementation for a KEDA external scaler
// which can be found at https://keda.sh/docs/2.0/concepts/external-scalers/
// This scaler has the implementation of an HTTP request counter and informs
// KEDA of the current request number for the queue in order to scale the app
package main

import (
	"fmt"
	"log"
	"net"

	externalscaler "github.com/kedacore/http-add-on/scaler/gen"
	"google.golang.org/grpc"
)

func main() {
	portStr := "8080"
	q := new(reqCounter)
	log.Fatal(startGrpcServer(portStr, q))
}

func startGrpcServer(port string, q httpQueue) error {
	lis, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%s", port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	grpcServer := grpc.NewServer()
	externalscaler.RegisterExternalScalerServer(grpcServer, newImpl(q))
	return grpcServer.Serve(lis)
}
