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

func TestMultipleRulesToSameBackend(t *testing.T) {
	t.Parallel()

	var app *h.TestApp

	feat := features.New("multi-rule-same-backend").
		WithLabel("area", "routing").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)

			app = f.CreateTestApp("shared-app", h.AppWithReplicas(1))

			// Single IR with two routing rules targeting the same backend:
			// one host-only rule and one host+path rule.
			// Note: avoid /api as the whoami container returns JSON on that path.
			f.CreateInterceptorRoute("ir-multi", app,
				h.IRWithRules(
					httpv1beta1.RoutingRule{
						Hosts: []string{f.Hostname("host-only")},
					},
					httpv1beta1.RoutingRule{
						Hosts: []string{f.Hostname("with-path")},
						Paths: []httpv1beta1.PathMatch{{Value: "/svc"}},
					},
				),
			)

			return ctx
		}).
		Assess("app is ready", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)
			f.WaitForReplicas(app, 1)
			return ctx
		}).
		Assess("host-only rule routes to the app", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)
			f.AssertRouteReachesApp(h.Request{Host: f.Hostname("host-only")}, "shared-app")
			return ctx
		}).
		Assess("host+path rule routes to the same app", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)
			f.AssertRouteReachesApp(h.Request{Host: f.Hostname("with-path"), Path: "/svc"}, "shared-app")
			return ctx
		}).
		Assess("unmatched path on path-rule host is rejected", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)
			f.AssertRouteRejected(h.Request{Host: f.Hostname("with-path"), Path: "/other"})
			return ctx
		}).
		Assess("unknown host is rejected", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)
			f.AssertRouteRejected(h.Request{Host: f.Hostname("unknown")})
			return ctx
		}).
		Feature()

	testenv.Test(t, feat)
}
