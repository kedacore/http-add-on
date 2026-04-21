//go:build e2e

package default_test

import (
	"context"
	"testing"

	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"

	h "github.com/kedacore/http-add-on/test/helpers"
)

func TestPortNameRouting(t *testing.T) {
	t.Parallel()

	feat := features.New("port-name-routing").
		WithLabel("area", "routing").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)

			app := f.CreateTestApp("portname-app", h.AppWithPortName("http"))

			ir := f.CreateInterceptorRoute("portname-ir", app,
				h.IRWithPortName("http"),
				h.IRWithHosts(f.Hostname()),
			)
			f.CreateScaledObject("portname-so", app, ir)

			return ctx
		}).
		Assess("routes request via named port", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)
			f.AssertRouteReachesApp(h.Request{Host: f.Hostname()}, "portname-app")
			return ctx
		}).
		Feature()

	testenv.Test(t, feat)
}
