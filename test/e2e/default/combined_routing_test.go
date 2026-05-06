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

func TestCombinedRouting(t *testing.T) {
	t.Parallel()

	feat := features.New("combined-routing").
		WithLabel("area", "routing").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)

			appA := f.CreateTestApp("app-a")
			appB := f.CreateTestApp("app-b")

			irA := f.CreateInterceptorRoute("ir-a", appA,
				h.IRWithRules(httpv1beta1.RoutingRule{
					Hosts: []string{f.Hostname("host-a")},
					Paths: []httpv1beta1.PathMatch{{Value: "/svc"}},
				}),
			)
			irB := f.CreateInterceptorRoute("ir-b", appB,
				h.IRWithRules(httpv1beta1.RoutingRule{
					Hosts: []string{f.Hostname("host-b")},
					Paths: []httpv1beta1.PathMatch{{Value: "/svc"}},
				}),
			)

			f.CreateScaledObject("so-a", appA, irA)
			f.CreateScaledObject("so-b", appB, irB)

			return ctx
		}).
		Assess("host-a /svc routes to app-a", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)
			f.AssertRouteReachesApp(h.Request{Host: f.Hostname("host-a"), Path: "/svc"}, "app-a")
			return ctx
		}).
		Assess("host-b /svc routes to app-b", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)
			f.AssertRouteReachesApp(h.Request{Host: f.Hostname("host-b"), Path: "/svc"}, "app-b")
			return ctx
		}).
		Assess("unknown host with /svc is rejected", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)
			f.AssertRouteRejected(h.Request{Host: f.Hostname("unknown"), Path: "/svc"})
			return ctx
		}).
		Feature()

	testenv.Test(t, feat)
}
