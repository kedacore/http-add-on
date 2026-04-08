//go:build e2e

package observability_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"

	h "github.com/kedacore/http-add-on/test/helpers"
)

const (
	interceptorMetricsService = "keda-add-ons-http-interceptor-metrics"
	interceptorMetricsPort    = "2223"
)

func TestPrometheusMetrics(t *testing.T) {
	t.Parallel()

	var app *h.TestApp

	feat := features.New("prometheus-metrics").
		WithLabel("area", "observability").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)

			app = f.CreateTestApp("prom-app")
			ir := f.CreateInterceptorRoute("prom-ir", app, h.IRWithHosts(f.Hostname()))
			f.CreateScaledObject("prom-so", app, ir)

			return ctx
		}).
		Assess("interceptor exposes request metrics", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)

			resp := f.ProxyRequest(h.Request{Host: f.Hostname()})
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("expected status 200, got %d", resp.StatusCode)
			}

			body, err := f.ServiceProxyGet(h.AddonNamespace, interceptorMetricsService, interceptorMetricsPort, "/metrics", nil)
			if err != nil {
				t.Fatalf("failed to get metrics: %v", err)
			}
			output := string(body)

			// Verify all interceptor metrics are present in the Prometheus output.
			for _, metric := range []string{
				"interceptor_pending_requests",
				"interceptor_request_duration_seconds",
				"interceptor_requests_total",
			} {
				if !strings.Contains(output, metric) {
					t.Fatalf("expected %s in metrics output, got:\n%s", metric, output)
				}
			}

			return ctx
		}).
		Feature()

	testenv.Test(t, feat)
}
