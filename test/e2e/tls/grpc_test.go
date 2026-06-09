//go:build e2e

package tls_test

import (
	"context"
	"crypto/tls"
	"testing"

	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"

	h "github.com/kedacore/http-add-on/test/helpers"
	echopb "github.com/kedacore/http-add-on/test/images/grpc-echo/proto"
)

// TestGRPCOverTLS verifies gRPC proxying over HTTP/2 with TLS (h2).
func TestGRPCOverTLS(t *testing.T) {
	t.Parallel()

	feat := features.New("grpc-over-tls").
		WithLabel("area", "tls").
		WithLabel("area", "grpc").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)

			appName := "grpc-tls-app"
			certSecretName := f.CreateCertificate([]string{appName})

			app := f.CreateTestApp(appName,
				h.AppWithTestImage(h.ImageGRPCEcho),
				h.AppWithAppProtocol(h.AppProtocolH2C),
				h.AppWithTLSSecret(certSecretName),
			)
			ir := f.CreateInterceptorRoute("grpc-tls-ir", app,
				h.IRWithHosts("grpc-tls."+tlsDomain),
			)
			f.CreateScaledObject("grpc-tls-so", app, ir)

			return ctx
		}).
		Assess("proxies a gRPC call over TLS", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)

			host := "grpc-tls." + tlsDomain
			client := f.GRPCEchoClient(host, &tls.Config{
				RootCAs:    f.CAPool,
				ServerName: host,
			})

			reply, err := client.Echo(ctx, &echopb.EchoRequest{Message: "tls-test"})
			if err != nil {
				t.Fatalf("gRPC Echo call over TLS failed: %v", err)
			}
			if reply.GetHostname() != "grpc-tls-app" {
				t.Errorf("expected hostname %q, got %q", "grpc-tls-app", reply.GetHostname())
			}
			if reply.GetProtocol() != "h2" {
				t.Errorf("expected protocol %q, got %q", "h2", reply.GetProtocol())
			}

			return ctx
		}).
		Feature()

	testenv.Test(t, feat)
}
