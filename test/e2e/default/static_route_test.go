//go:build e2e

package default_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"k8s.io/utils/ptr"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"

	httpv1beta1 "github.com/kedacore/http-add-on/operator/apis/http/v1beta1"
	h "github.com/kedacore/http-add-on/test/helpers"
)

func TestStaticRoute(t *testing.T) {
	t.Parallel()

	var app *h.TestApp

	feat := features.New("static-route").
		WithLabel("area", "routing").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)

			app = f.CreateTestApp("static-app")
			ir := f.CreateInterceptorRoute("static-ir", app,
				h.IRWithHosts(f.Hostname()),
				h.IRWithRequestRate(1),
				h.IRWithStaticRoutes(httpv1beta1.StaticRoute{
					Rules: []httpv1beta1.RoutingRule{
						{Paths: []httpv1beta1.PathMatch{{Value: "/healthz"}}},
					},
					Response: httpv1beta1.StaticResponse{
						StatusCode: http.StatusOK,
						Body:       ptr.To("healthy"),
						Headers:    map[string]string{"X-Static": "true"},
					},
				}),
			)
			f.CreateScaledObject("static-so", app, ir)

			return ctx
		}).
		Assess("serves static response when backend is not ready", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)

			resp := f.ProxyRequestRaw(h.Request{Host: f.Hostname(), Path: "/healthz"})

			if got, want := resp.StatusCode, http.StatusOK; got != want {
				t.Fatalf("status code = %d, want %d", got, want)
			}
			if got, want := string(resp.Body), "healthy"; got != want {
				t.Fatalf("body = %q, want %q", got, want)
			}
			if got, want := resp.Header.Get("X-Static"), "true"; got != want {
				t.Fatalf("X-Static = %q, want %q", got, want)
			}

			return ctx
		}).
		Assess("does not scale from static route traffic", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)

			f.AssertReplicasStable(app, 0, 10*time.Second)

			return ctx
		}).
		Assess("forwards to backend after scale-up", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)

			err := wait.For(func(_ context.Context) (bool, error) {
				resp := f.ProxyRequestRaw(h.Request{Host: f.Hostname()})
				return resp.StatusCode == http.StatusOK, nil
			}, wait.WithTimeout(2*time.Minute), wait.WithInterval(500*time.Millisecond))
			if err != nil {
				t.Fatal("timed out waiting for backend to become ready")
			}

			f.AssertRouteReachesApp(h.Request{Host: f.Hostname(), Path: "/healthz"}, "static-app")

			return ctx
		}).
		Feature()

	testenv.Test(t, feat)
}
