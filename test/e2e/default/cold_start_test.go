//go:build e2e

package default_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"

	httpv1beta1 "github.com/kedacore/http-add-on/operator/apis/http/v1beta1"
	h "github.com/kedacore/http-add-on/test/helpers"
)

func TestColdStart(t *testing.T) {
	t.Parallel()

	var app *h.TestApp

	feat := features.New("cold-start").
		WithLabel("area", "scaling").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)

			app = f.CreateTestApp("cold-start-app")
			ir := f.CreateInterceptorRoute("cold-start-ir", app,
				h.IRWithHosts(f.Hostname()),
			)
			f.CreateScaledObject("cold-start-so", app, ir)

			return ctx
		}).
		Assess("starts at zero replicas", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)

			f.WaitForReplicas(app, 0)

			return ctx
		}).
		Assess("first request triggers scale from zero", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)

			f.AssertStatus(h.Request{Host: f.Hostname()}, http.StatusOK)
			f.WaitForReplicas(app, 1)

			return ctx
		}).
		Assess("scales back to zero after cooldown", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)

			f.WaitForReplicas(app, 0)

			return ctx
		}).
		Feature()

	testenv.Test(t, feat)
}

func TestColdStartFallback(t *testing.T) {
	t.Parallel()

	feat := features.New("cold-start-fallback").
		WithLabel("area", "scaling").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)

			fallbackApp := f.CreateTestApp("fallback-app", h.AppWithReplicas(1))
			mainApp := f.CreateTestApp("main-app")

			// Use a short readiness timeout so the interceptor quickly falls back
			f.CreateInterceptorRoute("fallback-ir", mainApp,
				h.IRWithHosts(f.Hostname()),
				h.IRWithColdStart(fallbackApp.Name, fallbackApp.Port),
				h.IRWithTimeouts(httpv1beta1.InterceptorRouteTimeouts{
					Readiness: &metav1.Duration{Duration: 1 * time.Second},
				}),
			)
			// Skip the ScaledObject creation to trigger the fallback

			return ctx
		}).
		Assess("request is served by fallback when main app is not available", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)
			f.AssertRouteReachesApp(h.Request{Host: f.Hostname()}, "fallback-app")
			return ctx
		}).
		Feature()

	testenv.Test(t, feat)
}
