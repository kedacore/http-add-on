package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"

	pb "github.com/kedacore/http-add-on/test/images/grpc-echo/proto"
)

const (
	certFile = "/certs/tls.crt"
	keyFile  = "/certs/tls.key"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	hostname, _ := os.Hostname()

	grpcServer := grpc.NewServer()
	pb.RegisterEchoServiceServer(grpcServer, &echoServer{hostname: hostname})

	mux := http.NewServeMux()
	mux.Handle("/echo.EchoService/", grpcServer)
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	var protocols http.Protocols
	protocols.SetHTTP1(true)
	protocols.SetHTTP2(true)
	protocols.SetUnencryptedHTTP2(true)

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           mux,
		Protocols:         &protocols,
		ReadHeaderTimeout: time.Minute,
	}

	useTLS := hasTLSCerts()
	slog.Info("listening", "port", port, "tls", useTLS)

	var err error
	if useTLS {
		err = srv.ListenAndServeTLS(certFile, keyFile)
	} else {
		err = srv.ListenAndServe()
	}
	if errors.Is(err, http.ErrServerClosed) {
		slog.Info("server stopped gracefully")
		os.Exit(0)
	}

	slog.Error("server stopped", "err", err)
	os.Exit(1)
}

func hasTLSCerts() bool {
	_, certErr := os.Stat(certFile)
	_, keyErr := os.Stat(keyFile)
	return certErr == nil && keyErr == nil
}

type echoServer struct {
	pb.UnimplementedEchoServiceServer
	hostname string
}

func (s *echoServer) Echo(ctx context.Context, req *pb.EchoRequest) (*pb.EchoResponse, error) {
	proto := s.protocol(ctx)
	slog.Info("echo", "message", req.GetMessage(), "proto", proto)

	return &pb.EchoResponse{
		Message:  fmt.Sprintf("hello %s", req.GetMessage()),
		Hostname: s.hostname,
		Protocol: proto,
	}, nil
}

func (s *echoServer) EchoStream(stream grpc.BidiStreamingServer[pb.EchoRequest, pb.EchoResponse]) error {
	proto := s.protocol(stream.Context())
	slog.Info("echo stream started", "proto", proto)

	for {
		req, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			slog.Info("echo stream ended")
			return nil
		}
		if err != nil {
			return err
		}
		slog.Info("echo stream recv", "message", req.GetMessage())

		if err := stream.Send(&pb.EchoResponse{
			Message:  fmt.Sprintf("hello %s", req.GetMessage()),
			Hostname: s.hostname,
			Protocol: proto,
		}); err != nil {
			return err
		}
	}
}

func (s *echoServer) protocol(ctx context.Context) string {
	if p, ok := peer.FromContext(ctx); ok && p.AuthInfo != nil {
		return "h2"
	}
	return "h2c"
}
