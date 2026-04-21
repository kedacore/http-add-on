//go:build e2e

package default_test

import (
	"context"
	"testing"
	"time"

	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"

	httpv1beta1 "github.com/kedacore/http-add-on/operator/apis/http/v1beta1"
	h "github.com/kedacore/http-add-on/test/helpers"
)

func TestRouteDeletion(t *testing.T) {
	t.Parallel()

	var irA *httpv1beta1.InterceptorRoute

	feat := features.New("route-deletion").
		WithLabel("area", "routing").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)

			appA := f.CreateTestApp("app-a")
			appB := f.CreateTestApp("app-b")

			irA = f.CreateInterceptorRoute("ir-a", appA,
				h.IRWithHosts(f.Hostname("app-a")),
			)
			irB := f.CreateInterceptorRoute("ir-b", appB,
				h.IRWithHosts(f.Hostname("app-b")),
			)

			f.CreateScaledObject("so-a", appA, irA)
			f.CreateScaledObject("so-b", appB, irB)

			return ctx
		}).
		Assess("both routes work initially", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)
			f.AssertRouteReachesApp(h.Request{Host: f.Hostname("app-a")}, "app-a")
			f.AssertRouteReachesApp(h.Request{Host: f.Hostname("app-b")}, "app-b")
			return ctx
		}).
		Assess("deleted route is rejected while other route still works", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)

			f.DeleteResource(irA)

			// Wait for the IR to be fully removed from the API server,
			// then allow the interceptor's informer to propagate the deletion.
			if err := wait.For(
				conditions.New(cfg.Client().Resources()).ResourceDeleted(irA),
				wait.WithTimeout(2*time.Minute),
			); err != nil {
				t.Fatalf("InterceptorRoute was not deleted: %v", err)
			}
			time.Sleep(2 * time.Second)

			f.AssertRouteRejected(h.Request{Host: f.Hostname("app-a")})
			f.AssertRouteReachesApp(h.Request{Host: f.Hostname("app-b")}, "app-b")

			return ctx
		}).
		Feature()

	testenv.Test(t, feat)
}
