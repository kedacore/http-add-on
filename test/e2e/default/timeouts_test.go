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

func TestResponseHeaderTimeout(t *testing.T) {
	t.Parallel()

	defaultTimeout := features.New("default").
		WithLabel("area", "timeouts").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)

			app := f.CreateTestApp("default-app", h.AppWithReplicas(1))
			f.CreateInterceptorRoute("default-ir", app,
				h.IRWithHosts(f.Hostname()),
			)

			return ctx
		}).
		Assess("allows fast responses", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)
			f.AssertStatus(h.Request{Host: f.Hostname(), Query: "wait=100ms"}, http.StatusOK)
			return ctx
		}).
		Feature()

	shortTimeout := features.New("short").
		WithLabel("area", "timeouts").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)

			app := f.CreateTestApp("short-app", h.AppWithReplicas(1))
			f.CreateInterceptorRoute("short-ir", app,
				h.IRWithHosts(f.Hostname("short")),
				h.IRWithTimeouts(httpv1beta1.InterceptorRouteTimeouts{
					ResponseHeader: &metav1.Duration{Duration: 200 * time.Millisecond},
				}),
			)

			return ctx
		}).
		Assess("rejects slow backend with 504", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)
			f.AssertStatus(h.Request{Host: f.Hostname("short"), Query: "wait=5s"}, http.StatusGatewayTimeout)
			return ctx
		}).
		Feature()

	generousTimeout := features.New("generous").
		WithLabel("area", "timeouts").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)

			app := f.CreateTestApp("generous-app", h.AppWithReplicas(1))
			f.CreateInterceptorRoute("generous-ir", app,
				h.IRWithHosts(f.Hostname("generous")),
				h.IRWithTimeouts(httpv1beta1.InterceptorRouteTimeouts{
					ResponseHeader: &metav1.Duration{Duration: 10 * time.Second},
				}),
			)

			return ctx
		}).
		Assess("allows slow backend", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)
			f.AssertStatus(h.Request{Host: f.Hostname("generous"), Query: "wait=2s"}, http.StatusOK)
			return ctx
		}).
		Feature()

	testenv.Test(t, defaultTimeout, shortTimeout, generousTimeout)
}

func TestRequestTimeout(t *testing.T) {
	t.Parallel()

	withinDeadline := features.New("within-deadline").
		WithLabel("area", "timeouts").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)

			app := f.CreateTestApp("ok-app", h.AppWithReplicas(1))
			f.CreateInterceptorRoute("ok-ir", app,
				h.IRWithHosts(f.Hostname()),
				h.IRWithTimeouts(httpv1beta1.InterceptorRouteTimeouts{
					Request: &metav1.Duration{Duration: 10 * time.Second},
				}),
			)

			return ctx
		}).
		Assess("request within deadline succeeds", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)
			f.AssertStatus(h.Request{Host: f.Hostname(), Query: "wait=100ms"}, http.StatusOK)
			return ctx
		}).
		Feature()

	exceedingDeadline := features.New("exceeding-deadline").
		WithLabel("area", "timeouts").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)

			app := f.CreateTestApp("exceed-app", h.AppWithReplicas(1))
			f.CreateInterceptorRoute("exceed-ir", app,
				h.IRWithHosts(f.Hostname("exceed")),
				h.IRWithTimeouts(httpv1beta1.InterceptorRouteTimeouts{
					Request: &metav1.Duration{Duration: 500 * time.Millisecond},
				}),
			)

			return ctx
		}).
		Assess("request exceeding deadline is terminated with 504", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)
			f.AssertStatus(h.Request{Host: f.Hostname("exceed"), Query: "wait=5s"}, http.StatusGatewayTimeout)
			return ctx
		}).
		Feature()

	testenv.Test(t, withinDeadline, exceedingDeadline)
}

func TestReadinessTimeout(t *testing.T) {
	t.Parallel()

	generous := features.New("generous").
		WithLabel("area", "timeouts").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)

			app := f.CreateTestApp("generous-app")
			ir := f.CreateInterceptorRoute("generous-ir", app,
				h.IRWithHosts(f.Hostname()),
				h.IRWithTimeouts(httpv1beta1.InterceptorRouteTimeouts{
					Readiness: &metav1.Duration{Duration: 60 * time.Second},
				}),
			)
			f.CreateScaledObject("generous-so", app, ir)

			return ctx
		}).
		Assess("cold start completes within generous timeout", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)
			f.AssertStatus(h.Request{Host: f.Hostname()}, http.StatusOK)
			return ctx
		}).
		Feature()

	short := features.New("short").
		WithLabel("area", "timeouts").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)

			app := f.CreateTestApp("short-app")
			ir := f.CreateInterceptorRoute("short-ir", app,
				h.IRWithHosts(f.Hostname("short")),
				h.IRWithTimeouts(httpv1beta1.InterceptorRouteTimeouts{
					Readiness: &metav1.Duration{Duration: 1 * time.Millisecond},
				}),
			)
			f.CreateScaledObject("short-so", app, ir)

			return ctx
		}).
		Assess("returns 504 when backend is not ready in time", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)
			f.AssertStatus(h.Request{Host: f.Hostname("short")}, http.StatusGatewayTimeout)
			return ctx
		}).
		Feature()

	testenv.Test(t, generous, short)
}
