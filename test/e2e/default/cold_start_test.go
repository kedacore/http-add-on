//go:build e2e

package default_test

import (
	"context"
	"net/http"
	"testing"

	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"

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

			resp := f.ProxyRequest(h.Request{Host: f.Hostname()})
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("expected status 200, got %d; body: %s", resp.StatusCode, string(resp.Body))
			}

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
