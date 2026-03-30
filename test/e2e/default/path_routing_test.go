//go:build e2e

package default_test

import (
	"context"
	"testing"

	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"

	httpv1beta1 "github.com/kedacore/http-add-on/operator/apis/http/v1beta1"
	h "github.com/kedacore/http-add-on/test/helpers"
)

func TestPathRouting(t *testing.T) {
	t.Parallel()

	feat := features.New("path-routing").
		WithLabel("area", "routing").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)

			appA := f.CreateTestApp("app-a")
			appB := f.CreateTestApp("app-b")

			irA := f.CreateInterceptorRoute("ir-a", appA,
				h.IRWithRules(httpv1beta1.RoutingRule{
					Hosts: []string{f.Hostname()},
					Paths: []httpv1beta1.PathMatch{{Value: "/svc-a"}},
				}),
			)
			irB := f.CreateInterceptorRoute("ir-b", appB,
				h.IRWithRules(httpv1beta1.RoutingRule{
					Hosts: []string{f.Hostname()},
					Paths: []httpv1beta1.PathMatch{{Value: "/svc-b"}},
				}),
			)

			f.CreateScaledObject("so-a", appA, irA)
			f.CreateScaledObject("so-b", appB, irB)

			return ctx
		}).
		Assess("request to /svc-a routes to app-a", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)

			resp := f.ProxyRequest(h.Request{Host: f.Hostname(), Path: "/svc-a"})

			if resp.StatusCode != 200 {
				t.Fatalf("expected status 200, got %d; body: %s", resp.StatusCode, resp.Body)
			}
			if resp.Hostname != "app-a" {
				t.Errorf("expected request to be served by app-a, got %q", resp.Hostname)
			}
			if resp.RequestPath != "/svc-a" {
				t.Errorf("expected path /svc-a, got %q", resp.RequestPath)
			}

			return ctx
		}).
		Assess("request to non-matching path is rejected", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)

			resp := f.ProxyRequest(h.Request{Host: f.Hostname(), Path: "/unknown"})

			if resp.StatusCode == 200 {
				t.Fatalf("expected non-200 status for unmatched path, got 200; body: %s", resp.Body)
			}

			return ctx
		}).
		Assess("request to /svc-b routes to app-b", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)

			resp := f.ProxyRequest(h.Request{Host: f.Hostname(), Path: "/svc-b"})

			if resp.StatusCode != 200 {
				t.Fatalf("expected status 200, got %d; body: %s", resp.StatusCode, resp.Body)
			}
			if resp.Hostname != "app-b" {
				t.Errorf("expected request to be served by app-b, got %q", resp.Hostname)
			}
			if resp.RequestPath != "/svc-b" {
				t.Errorf("expected path /svc-b, got %q", resp.RequestPath)
			}

			return ctx
		}).
		Feature()

	testenv.Test(t, feat)
}
