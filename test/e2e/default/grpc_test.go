//go:build e2e

package default_test

import (
	"context"
	"errors"
	"io"
	"testing"

	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"

	h "github.com/kedacore/http-add-on/test/helpers"
	echopb "github.com/kedacore/http-add-on/test/images/grpc-echo/proto"
)

// TestGRPC verifies gRPC proxying over HTTP/2 cleartext (h2c).
func TestGRPC(t *testing.T) {
	t.Parallel()

	feat := features.New("grpc").
		WithLabel("area", "grpc").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)

			app := f.CreateTestApp("grpc-app",
				h.AppWithTestImage(h.ImageGRPCEcho),
			)
			ir := f.CreateInterceptorRoute("grpc-ir", app,
				h.IRWithHosts(f.Hostname()),
			)
			f.CreateScaledObject("grpc-so", app, ir)

			return ctx
		}).
		Assess("proxies a gRPC unary call", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)

			client := f.GRPCEchoClient(f.Hostname(), nil)

			reply, err := client.Echo(ctx, &echopb.EchoRequest{Message: "test"})
			if err != nil {
				t.Fatalf("gRPC Echo call failed: %v", err)
			}
			if reply.GetHostname() != "grpc-app" {
				t.Errorf("expected hostname %q, got %q", "grpc-app", reply.GetHostname())
			}
			if reply.GetProtocol() != "h2c" {
				t.Errorf("expected protocol %q, got %q", "h2c", reply.GetProtocol())
			}

			return ctx
		}).
		Assess("proxies a gRPC bidirectional stream", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)

			client := f.GRPCEchoClient(f.Hostname(), nil)
			stream, err := client.EchoStream(ctx)
			if err != nil {
				t.Fatalf("failed to open stream: %v", err)
			}

			for _, msg := range []string{"alice", "bob", "charlie"} {
				if err := stream.Send(&echopb.EchoRequest{Message: msg}); err != nil {
					t.Fatalf("failed to send %q: %v", msg, err)
				}
				reply, err := stream.Recv()
				if err != nil {
					t.Fatalf("failed to recv after sending %q: %v", msg, err)
				}
				want := "hello " + msg
				if reply.GetMessage() != want {
					t.Errorf("expected message %q, got %q", want, reply.GetMessage())
				}
			}

			if err := stream.CloseSend(); err != nil {
				t.Fatalf("failed to close send: %v", err)
			}
			if _, err := stream.Recv(); !errors.Is(err, io.EOF) {
				t.Fatalf("expected EOF after CloseSend, got: %v", err)
			}

			return ctx
		}).
		Feature()

	testenv.Test(t, feat)
}
