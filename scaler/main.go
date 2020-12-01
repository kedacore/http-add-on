package main

import (
	"fmt"
	"log"
	"net"
	"os"

	externalscaler "github.com/kedacore/http-add-on/scaler/gen"
	"google.golang.org/grpc"
)

func main() {
	portStr := os.Getenv("PORT")
	q := new(reqCounter)
	log.Fatal(startGrpcServer(portStr, q))
}

func startGrpcServer(port string, q httpQueue) error {
	lis, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0%s", port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	grpcServer := grpc.NewServer()
	externalscaler.RegisterExternalScalerServer(grpcServer, newImpl(q))
	return grpcServer.Serve(lis)
}
