//go:build e2e

package default_test

import (
	"context"
	"net/http"
	"testing"

	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"

	"github.com/kedacore/http-add-on/pkg/k8s"
	h "github.com/kedacore/http-add-on/test/helpers"
)

func TestColdStartFallback(t *testing.T) {
	t.Parallel()

	feat := features.New("cold-start-fallback").
		WithLabel("area", "scaling").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)

			fallbackApp := f.CreateTestApp("fallback-app", h.AppWithReplicas(1))
			mainApp := f.CreateTestApp("main-app")

			// Use a short condition wait timeout so the interceptor quickly falls back
			f.CreateInterceptorRoute("fallback-ir", mainApp,
				h.IRWithHosts(f.Hostname()),
				h.IRWithColdStart(fallbackApp.Name, fallbackApp.Port),

				// TODO: update this test to use actual timeouts instead of the internal annotation
				h.IRWithAnnotations(map[string]string{
					k8s.AnnotationConditionWaitTimeout: "1s",
				}),
			)
			// Skip the ScaledObject creation to trigger the fallback

			return ctx
		}).
		Assess("request is served by fallback when main app is not available", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)

			resp := f.ProxyRequest(h.Request{Host: f.Hostname()})
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("expected status 200, got %d; body: %s", resp.StatusCode, resp.Body)
			}
			if resp.Hostname != "fallback-app" {
				t.Errorf("expected fallback-app, got %q", resp.Hostname)
			}

			return ctx
		}).
		Feature()

	testenv.Test(t, feat)
}
