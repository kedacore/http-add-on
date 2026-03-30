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

func TestHostRouting(t *testing.T) {
	t.Parallel()

	feat := features.New("host-routing").
		WithLabel("area", "routing").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)

			app := f.CreateTestApp("test-app")
			ir := f.CreateInterceptorRoute("test-ir", app,
				h.IRWithHosts(f.Hostname()),
			)
			f.CreateScaledObject("test-so", app, ir)

			return ctx
		}).
		Assess("routes request to correct backend", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)

			resp := f.ProxyRequest(h.Request{Host: f.Hostname()})
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("expected status 200, got %d; body: %s", resp.StatusCode, string(resp.Body))
			}
			if resp.Hostname != "test-app" {
				t.Errorf("expected request to be served by test-app, got %q", resp.Hostname)
			}

			return ctx
		}).
		Assess("rejects request for unknown host", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)

			resp := f.ProxyRequest(h.Request{Host: f.Hostname("unknown")})
			if resp.StatusCode == http.StatusOK {
				t.Fatalf("expected non-200 status for unknown host, got 200; body: %s", resp.Body)
			}

			return ctx
		}).
		Feature()

	testenv.Test(t, feat)
}

func TestMultipleHostsRouting(t *testing.T) {
	t.Parallel()

	feat := features.New("multiple-hosts-routing").
		WithLabel("area", "routing").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)

			app := f.CreateTestApp("multi-host-app")
			ir := f.CreateInterceptorRoute("multi-host-ir", app,
				h.IRWithHosts(f.Hostname("host-a"), f.Hostname("host-b")),
			)
			f.CreateScaledObject("multi-host-so", app, ir)

			return ctx
		}).
		Assess("routes host-a to the app", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)

			resp := f.ProxyRequest(h.Request{Host: f.Hostname("host-a")})
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("expected status 200 for host-a, got %d; body: %s", resp.StatusCode, resp.Body)
			}
			if resp.Hostname != "multi-host-app" {
				t.Errorf("expected request to be served by multi-host-app, got %q", resp.Hostname)
			}

			return ctx
		}).
		Assess("routes host-b to the same app", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)

			resp := f.ProxyRequest(h.Request{Host: f.Hostname("host-b")})
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("expected status 200 for host-b, got %d; body: %s", resp.StatusCode, resp.Body)
			}
			if resp.Hostname != "multi-host-app" {
				t.Errorf("expected request to be served by multi-host-app, got %q", resp.Hostname)
			}

			return ctx
		}).
		Assess("rejects request for unlisted host", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)

			resp := f.ProxyRequest(h.Request{Host: f.Hostname("host-c")})
			if resp.StatusCode == http.StatusOK {
				t.Fatalf("expected non-200 status for unlisted host, got 200; body: %s", resp.Body)
			}

			return ctx
		}).
		Feature()

	testenv.Test(t, feat)
}

func TestWildcardHostRouting(t *testing.T) {
	t.Parallel()

	feat := features.New("wildcard-host-routing").
		WithLabel("area", "routing").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)

			specificApp := f.CreateTestApp("specific-app")
			wildcardApp := f.CreateTestApp("wildcard-app")

			irExact := f.CreateInterceptorRoute("ir-exact", specificApp,
				h.IRWithHosts(f.Hostname("specific")),
			)
			irWildcard := f.CreateInterceptorRoute("ir-wildcard", wildcardApp,
				h.IRWithRules(httpv1beta1.RoutingRule{
					Hosts: []string{f.Hostname("*")},
				}),
			)

			f.CreateScaledObject("so-exact", specificApp, irExact)
			f.CreateScaledObject("so-wildcard", wildcardApp, irWildcard)

			return ctx
		}).
		Assess("exact host matches exact route", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)

			resp := f.ProxyRequest(h.Request{Host: f.Hostname("specific")})
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("expected status 200, got %d; body: %s", resp.StatusCode, resp.Body)
			}
			if resp.Hostname != "specific-app" {
				t.Errorf("expected specific-app, got %q", resp.Hostname)
			}

			return ctx
		}).
		Assess("other subdomain matches wildcard route", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)

			resp := f.ProxyRequest(h.Request{Host: f.Hostname("anything")})
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("expected status 200, got %d; body: %s", resp.StatusCode, resp.Body)
			}
			if resp.Hostname != "wildcard-app" {
				t.Errorf("expected wildcard-app, got %q", resp.Hostname)
			}

			return ctx
		}).
		Assess("non-matching domain is rejected", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)

			resp := f.ProxyRequest(h.Request{Host: "unmatched.other.test"})
			if resp.StatusCode == http.StatusOK {
				t.Fatalf("expected non-200 for non-matching domain, got 200; body: %s", resp.Body)
			}

			return ctx
		}).
		Feature()

	testenv.Test(t, feat)
}
