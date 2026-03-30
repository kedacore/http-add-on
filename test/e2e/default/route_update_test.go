//go:build e2e

package default_test

import (
	"context"
	"net/http"
	"testing"

	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"

	httpv1beta1 "github.com/kedacore/http-add-on/operator/apis/http/v1beta1"
	h "github.com/kedacore/http-add-on/test/helpers"
)

func TestRouteUpdate(t *testing.T) {
	t.Parallel()

	var ir *httpv1beta1.InterceptorRoute

	feat := features.New("route-update").
		WithLabel("area", "routing").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)

			appTarget := f.CreateTestApp("app-a")
			_ = f.CreateTestApp("app-b", h.AppWithReplicas(1))
			ir = f.CreateInterceptorRoute("update-ir", appTarget,
				h.IRWithHosts(f.Hostname()),
			)
			f.CreateScaledObject("update-so", appTarget, ir)

			return ctx
		}).
		Assess("initially routes to app-a", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)

			resp := f.ProxyRequest(h.Request{Host: f.Hostname()})
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("expected status 200, got %d; body: %s", resp.StatusCode, resp.Body)
			}
			if resp.Hostname != "app-a" {
				t.Errorf("expected app-a, got %q", resp.Hostname)
			}

			return ctx
		}).
		Assess("routes to app-b after IR update", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)

			f.UpdateInterceptorRoute(ir, func(ir *httpv1beta1.InterceptorRoute) {
				ir.Spec.Target.Service = "app-b"
			})

			resp := f.ProxyRequest(h.Request{Host: f.Hostname()})
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("expected status 200, got %d; body: %s", resp.StatusCode, resp.Body)
			}
			if resp.Hostname != "app-b" {
				t.Errorf("expected app-b after update, got %q", resp.Hostname)
			}

			return ctx
		}).
		Feature()

	testenv.Test(t, feat)
}
