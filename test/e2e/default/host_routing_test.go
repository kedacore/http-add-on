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
			f.AssertRouteReachesApp(h.Request{Host: f.Hostname()}, "test-app")
			return ctx
		}).
		Assess("rejects request for unknown host", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)
			f.AssertRouteRejected(h.Request{Host: f.Hostname("unknown")})
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
			f.AssertRouteReachesApp(h.Request{Host: f.Hostname("host-a")}, "multi-host-app")
			return ctx
		}).
		Assess("routes host-b to the same app", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)
			f.AssertRouteReachesApp(h.Request{Host: f.Hostname("host-b")}, "multi-host-app")
			return ctx
		}).
		Assess("rejects request for unlisted host", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)
			f.AssertRouteRejected(h.Request{Host: f.Hostname("host-c")})
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
			f.AssertRouteReachesApp(h.Request{Host: f.Hostname("specific")}, "specific-app")
			return ctx
		}).
		Assess("other subdomain matches wildcard route", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)
			f.AssertRouteReachesApp(h.Request{Host: f.Hostname("anything")}, "wildcard-app")
			return ctx
		}).
		Assess("non-matching domain is rejected", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)
			f.AssertRouteRejected(h.Request{Host: "unmatched.other.test"})
			return ctx
		}).
		Feature()

	testenv.Test(t, feat)
}
