//go:build e2e

package tls_test

import (
	"context"
	"crypto/tls"
	"net/http"
	"testing"

	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"

	h "github.com/kedacore/http-add-on/test/helpers"
)

func TestTLSTermination(t *testing.T) {
	t.Parallel()

	feat := features.New("tls-termination").
		WithLabel("area", "tls").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)

			appName := "tls-app"
			certSecretName := f.CreateCertificate([]string{appName})

			app := f.CreateTestApp(appName, h.AppWithTLSSecret(certSecretName))
			ir := f.CreateInterceptorRoute("tls-ir", app, h.IRWithHosts("tls-test."+tlsDomain))
			f.CreateScaledObject("tls-so", app, ir)

			return ctx
		}).
		Assess("handles TLS request", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)

			host := "tls-test." + tlsDomain
			resp := f.ProxyRequest(h.Request{
				Host: host,
				TLSConfig: &tls.Config{
					RootCAs:    f.CAPool,
					ServerName: host,
				},
			})

			if resp.StatusCode != http.StatusOK {
				t.Fatalf("expected status 200, got %d; body: %s", resp.StatusCode, resp.Body)
			}
			if resp.Hostname != "tls-app" {
				t.Errorf("expected request to be served by tls-app, got %q", resp.Hostname)
			}

			return ctx
		}).
		Feature()

	testenv.Test(t, feat)
}
