//go:build e2e

package default_test

import (
	"context"
	"net/http"
	"testing"

	"k8s.io/utils/ptr"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"

	httpv1beta1 "github.com/kedacore/http-add-on/operator/apis/http/v1beta1"
	h "github.com/kedacore/http-add-on/test/helpers"
)

func TestHeaderRouting(t *testing.T) {
	t.Parallel()

	feat := features.New("header-routing").
		WithLabel("area", "routing").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)

			appFoo := f.CreateTestApp("app-foo")
			appBar := f.CreateTestApp("app-bar")

			irFoo := f.CreateInterceptorRoute("ir-foo", appFoo,
				h.IRWithRules(httpv1beta1.RoutingRule{
					Headers: []httpv1beta1.HeaderMatch{{
						Name:  "X-Route",
						Value: ptr.To("foo"),
					}},
				}),
			)
			irBar := f.CreateInterceptorRoute("ir-bar", appBar,
				h.IRWithRules(httpv1beta1.RoutingRule{
					Headers: []httpv1beta1.HeaderMatch{{
						Name:  "X-Route",
						Value: ptr.To("bar"),
					}},
				}),
			)

			f.CreateScaledObject("so-foo", appFoo, irFoo)
			f.CreateScaledObject("so-bar", appBar, irBar)

			return ctx
		}).
		Assess("foo routes to app-foo", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)

			resp := f.ProxyRequest(h.Request{
				Host:    f.Hostname(),
				Headers: map[string]string{"X-Route": "foo"},
			})
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("expected status 200, got %d; body: %s", resp.StatusCode, resp.Body)
			}
			if resp.Hostname != "app-foo" {
				t.Errorf("expected request to be served by app-foo, got %q", resp.Hostname)
			}

			return ctx
		}).
		Assess("bar routes to app-bar", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)

			resp := f.ProxyRequest(h.Request{
				Host:    f.Hostname(),
				Headers: map[string]string{"X-Route": "bar"},
			})
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("expected status 200, got %d; body: %s", resp.StatusCode, resp.Body)
			}
			if resp.Hostname != "app-bar" {
				t.Errorf("expected request to be served by app-bar, got %q", resp.Hostname)
			}

			return ctx
		}).
		Assess("request without header is rejected", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)

			resp := f.ProxyRequest(h.Request{
				Host: f.Hostname(),
			})
			if resp.StatusCode == http.StatusOK {
				t.Fatalf("expected non-200 status for missing header, got 200; body: %s", resp.Body)
			}

			return ctx
		}).
		Feature()

	testenv.Test(t, feat)
}

func TestHeaderPresenceRouting(t *testing.T) {
	t.Parallel()

	feat := features.New("header-presence-routing").
		WithLabel("area", "routing").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)

			presenceApp := f.CreateTestApp("presence-app")
			exactApp := f.CreateTestApp("exact-app")

			// Route with exact header value match - more specific, should win when value matches.
			irExact := f.CreateInterceptorRoute("ir-exact-header", exactApp,
				h.IRWithRules(httpv1beta1.RoutingRule{
					Hosts: []string{f.Hostname()},
					Headers: []httpv1beta1.HeaderMatch{{
						Name:  "X-Debug",
						Value: ptr.To("verbose"),
					}},
				}),
			)
			// Route with header presence only - matches any value for X-Debug.
			irPresence := f.CreateInterceptorRoute("ir-presence-header", presenceApp,
				h.IRWithRules(httpv1beta1.RoutingRule{
					Hosts: []string{f.Hostname()},
					Headers: []httpv1beta1.HeaderMatch{{
						Name: "X-Debug",
					}},
				}),
			)

			f.CreateScaledObject("so-exact-header", exactApp, irExact)
			f.CreateScaledObject("so-presence-header", presenceApp, irPresence)

			return ctx
		}).
		Assess("exact header value matches exact route", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)

			resp := f.ProxyRequest(h.Request{
				Host:    f.Hostname(),
				Headers: map[string]string{"X-Debug": "verbose"},
			})
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("expected status 200, got %d; body: %s", resp.StatusCode, resp.Body)
			}
			if resp.Hostname != "exact-app" {
				t.Errorf("expected exact-app, got %q", resp.Hostname)
			}

			return ctx
		}).
		Assess("other header value matches presence route", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)

			resp := f.ProxyRequest(h.Request{
				Host:    f.Hostname(),
				Headers: map[string]string{"X-Debug": "minimal"},
			})
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("expected status 200, got %d; body: %s", resp.StatusCode, resp.Body)
			}
			if resp.Hostname != "presence-app" {
				t.Errorf("expected presence-app, got %q", resp.Hostname)
			}

			return ctx
		}).
		Feature()

	testenv.Test(t, feat)
}
