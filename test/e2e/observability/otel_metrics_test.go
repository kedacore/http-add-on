//go:build e2e

package observability_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"

	h "github.com/kedacore/http-add-on/test/helpers"
)

const (
	otelNamespace   = "open-telemetry-system"
	otelServiceName = "opentelemetry-collector"
	otelPromPort    = "prom-exporter"
)

func TestOtelMetrics(t *testing.T) {
	t.Parallel()

	var app *h.TestApp

	feat := features.New("otel-metrics").
		WithLabel("area", "observability").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)

			app = f.CreateTestApp("otel-metrics-app")
			ir := f.CreateInterceptorRoute("otel-metrics-ir", app, h.IRWithHosts(f.Hostname()))
			f.CreateScaledObject("otel-metrics-so", app, ir)

			return ctx
		}).
		Assess("otel collector receives interceptor metrics", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			f := h.NewFramework(ctx, t)

			resp := f.ProxyRequest(h.Request{Host: f.Hostname()})
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("expected status 200, got %d", resp.StatusCode)
			}

			// Poll the collector's Prometheus endpoint - metrics may take a moment to arrive.
			err := wait.For(func(_ context.Context) (bool, error) {
				body, err := f.ServiceProxyGet(otelNamespace, otelServiceName, otelPromPort, "/metrics", nil)
				if err != nil {
					return false, nil
				}
				return strings.Contains(string(body), "interceptor_request_count_total"), nil
			}, wait.WithTimeout(2*time.Minute), wait.WithInterval(5*time.Second))
			if err != nil {
				t.Fatal("interceptor_request_count_total not found in OTEL collector metrics")
			}

			return ctx
		}).
		Feature()

	testenv.Test(t, feat)
}
