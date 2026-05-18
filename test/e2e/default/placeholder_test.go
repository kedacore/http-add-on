//go:build e2e

package default_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"

	httpv1beta1 "github.com/kedacore/http-add-on/operator/apis/http/v1beta1"
	h "github.com/kedacore/http-add-on/test/helpers"
)

func TestColdStartPlaceholder(t *testing.T) {
	t.Parallel()

	feat := features.New("cold-start-placeholder").
		WithLabel("area", "scaling").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)

			app := f.CreateTestApp("placeholder-app")
			ir := f.CreateInterceptorRoute("placeholder-ir", app,
				h.IRWithHosts(f.Hostname()),
				h.IRWithRequestRate(1),
				h.IRWithPlaceholderResponse(httpv1beta1.StaticResponse{
					StatusCode: http.StatusServiceUnavailable,
					Body:       ptr.To(`{"status":"loading"}`),
					Headers: map[string]string{
						"Content-Type": "application/json",
					},
				}),
			)
			f.CreateScaledObject("placeholder-so", app, ir)

			return ctx
		}).
		Assess("returns placeholder response when backend is not ready", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)

			resp := f.ProxyRequestRaw(h.Request{Host: f.Hostname()})

			if got, want := resp.StatusCode, http.StatusServiceUnavailable; got != want {
				t.Fatalf("status code = %d, want %d", got, want)
			}
			if got, want := string(resp.Body), `{"status":"loading"}`; got != want {
				t.Fatalf("body = %q, want %q", got, want)
			}
			if got, want := resp.Header.Get("Content-Type"), "application/json"; got != want {
				t.Fatalf("Content-Type = %q, want %q", got, want)
			}

			return ctx
		}).
		Assess("serves real response after backend scales up", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)

			err := wait.For(func(_ context.Context) (bool, error) {
				resp := f.ProxyRequestRaw(h.Request{Host: f.Hostname()})
				return resp.StatusCode == http.StatusOK, nil
			}, wait.WithTimeout(2*time.Minute), wait.WithInterval(500*time.Millisecond))
			if err != nil {
				t.Fatal("timed out waiting for backend to become ready")
			}

			f.AssertRouteReachesApp(h.Request{Host: f.Hostname()}, "placeholder-app")

			return ctx
		}).
		Feature()

	testenv.Test(t, feat)
}

func TestColdStartPlaceholderWithFallback(t *testing.T) {
	t.Parallel()

	feat := features.New("cold-start-placeholder-with-fallback").
		WithLabel("area", "scaling").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)

			fallbackApp := f.CreateTestApp("fallback-app", h.AppWithReplicas(1))
			mainApp := f.CreateTestApp("main-app")

			f.CreateInterceptorRoute("placeholder-fallback-ir", mainApp,
				h.IRWithHosts(f.Hostname()),
				h.IRWithPlaceholderResponse(httpv1beta1.StaticResponse{
					StatusCode: http.StatusTeapot,
				}),
				h.IRWithColdStart(fallbackApp.Name, fallbackApp.Port),
				h.IRWithTimeouts(httpv1beta1.InterceptorRouteTimeouts{
					Readiness: &metav1.Duration{Duration: 1 * time.Second},
				}),
			)

			return ctx
		}).
		Assess("returns placeholder response, not fallback", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)

			resp := f.ProxyRequestRaw(h.Request{Host: f.Hostname()})

			if got, want := resp.StatusCode, http.StatusTeapot; got != want {
				t.Fatalf("status code = %d, want %d", got, want)
			}

			return ctx
		}).
		Feature()

	testenv.Test(t, feat)
}
