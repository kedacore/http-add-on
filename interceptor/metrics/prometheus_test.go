package metrics

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	promexporter "go.opentelemetry.io/otel/exporters/prometheus"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"

	"github.com/kedacore/http-add-on/interceptor/config"
)

func testRegistry(t *testing.T) (*prometheus.Registry, *Instruments) {
	t.Helper()

	registry := prometheus.NewRegistry()
	exporter, err := promexporter.New(
		promexporter.WithRegisterer(registry),
		promexporter.WithoutScopeInfo(),
	)
	if err != nil {
		t.Fatalf("creating prometheus exporter: %v", err)
	}

	provider, err := NewMeterProvider(
		config.Metrics{OtelPrometheusExporterEnabled: false},
		sdkmetric.WithReader(exporter),
	)
	if err != nil {
		t.Fatalf("NewMeterProvider() error: %v", err)
	}
	t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })

	instruments, err := NewInstruments(provider)
	if err != nil {
		t.Fatalf("NewInstruments() error: %v", err)
	}

	return registry, instruments
}

func TestPrometheus_RequestMetrics(t *testing.T) {
	registry, instruments := testRegistry(t)

	instruments.RecordRequest("GET", 200, "my-route", "my-ns", 100*time.Millisecond)
	instruments.RecordRequest("POST", 500, "my-route", "my-ns", 200*time.Millisecond)

	expected := `
		# HELP interceptor_request_count_total Total requests processed by the interceptor proxy
		# TYPE interceptor_request_count_total counter
		interceptor_request_count_total{code="200",method="GET",route_name="my-route",route_namespace="my-ns"} 1
		interceptor_request_count_total{code="500",method="POST",route_name="my-route",route_namespace="my-ns"} 1
	`
	if err := testutil.CollectAndCompare(registry, strings.NewReader(expected), "interceptor_request_count_total"); err != nil {
		t.Fatalf("unexpected metrics output:\n%v", err)
	}
}

func TestPrometheus_DurationMetrics(t *testing.T) {
	registry, instruments := testRegistry(t)

	instruments.RecordRequest("GET", 200, "app-alpha", "production", 30*time.Millisecond)
	instruments.RecordRequest("GET", 200, "app-alpha", "production", 3*time.Second)
	instruments.RecordRequest("POST", 503, "app-beta", "staging", 12*time.Second)

	expected := `
		# HELP interceptor_request_duration_seconds Time from request received to response written
		# TYPE interceptor_request_duration_seconds histogram
		interceptor_request_duration_seconds_bucket{code="200",method="GET",route_name="app-alpha",route_namespace="production",le="0.005"} 0
		interceptor_request_duration_seconds_bucket{code="200",method="GET",route_name="app-alpha",route_namespace="production",le="0.01"} 0
		interceptor_request_duration_seconds_bucket{code="200",method="GET",route_name="app-alpha",route_namespace="production",le="0.025"} 0
		interceptor_request_duration_seconds_bucket{code="200",method="GET",route_name="app-alpha",route_namespace="production",le="0.05"} 1
		interceptor_request_duration_seconds_bucket{code="200",method="GET",route_name="app-alpha",route_namespace="production",le="0.075"} 1
		interceptor_request_duration_seconds_bucket{code="200",method="GET",route_name="app-alpha",route_namespace="production",le="0.1"} 1
		interceptor_request_duration_seconds_bucket{code="200",method="GET",route_name="app-alpha",route_namespace="production",le="0.25"} 1
		interceptor_request_duration_seconds_bucket{code="200",method="GET",route_name="app-alpha",route_namespace="production",le="0.5"} 1
		interceptor_request_duration_seconds_bucket{code="200",method="GET",route_name="app-alpha",route_namespace="production",le="0.75"} 1
		interceptor_request_duration_seconds_bucket{code="200",method="GET",route_name="app-alpha",route_namespace="production",le="1"} 1
		interceptor_request_duration_seconds_bucket{code="200",method="GET",route_name="app-alpha",route_namespace="production",le="2.5"} 1
		interceptor_request_duration_seconds_bucket{code="200",method="GET",route_name="app-alpha",route_namespace="production",le="5"} 2
		interceptor_request_duration_seconds_bucket{code="200",method="GET",route_name="app-alpha",route_namespace="production",le="7.5"} 2
		interceptor_request_duration_seconds_bucket{code="200",method="GET",route_name="app-alpha",route_namespace="production",le="10"} 2
		interceptor_request_duration_seconds_bucket{code="200",method="GET",route_name="app-alpha",route_namespace="production",le="+Inf"} 2
		interceptor_request_duration_seconds_sum{code="200",method="GET",route_name="app-alpha",route_namespace="production"} 3.03
		interceptor_request_duration_seconds_count{code="200",method="GET",route_name="app-alpha",route_namespace="production"} 2
		interceptor_request_duration_seconds_bucket{code="503",method="POST",route_name="app-beta",route_namespace="staging",le="0.005"} 0
		interceptor_request_duration_seconds_bucket{code="503",method="POST",route_name="app-beta",route_namespace="staging",le="0.01"} 0
		interceptor_request_duration_seconds_bucket{code="503",method="POST",route_name="app-beta",route_namespace="staging",le="0.025"} 0
		interceptor_request_duration_seconds_bucket{code="503",method="POST",route_name="app-beta",route_namespace="staging",le="0.05"} 0
		interceptor_request_duration_seconds_bucket{code="503",method="POST",route_name="app-beta",route_namespace="staging",le="0.075"} 0
		interceptor_request_duration_seconds_bucket{code="503",method="POST",route_name="app-beta",route_namespace="staging",le="0.1"} 0
		interceptor_request_duration_seconds_bucket{code="503",method="POST",route_name="app-beta",route_namespace="staging",le="0.25"} 0
		interceptor_request_duration_seconds_bucket{code="503",method="POST",route_name="app-beta",route_namespace="staging",le="0.5"} 0
		interceptor_request_duration_seconds_bucket{code="503",method="POST",route_name="app-beta",route_namespace="staging",le="0.75"} 0
		interceptor_request_duration_seconds_bucket{code="503",method="POST",route_name="app-beta",route_namespace="staging",le="1"} 0
		interceptor_request_duration_seconds_bucket{code="503",method="POST",route_name="app-beta",route_namespace="staging",le="2.5"} 0
		interceptor_request_duration_seconds_bucket{code="503",method="POST",route_name="app-beta",route_namespace="staging",le="5"} 0
		interceptor_request_duration_seconds_bucket{code="503",method="POST",route_name="app-beta",route_namespace="staging",le="7.5"} 0
		interceptor_request_duration_seconds_bucket{code="503",method="POST",route_name="app-beta",route_namespace="staging",le="10"} 0
		interceptor_request_duration_seconds_bucket{code="503",method="POST",route_name="app-beta",route_namespace="staging",le="+Inf"} 1
		interceptor_request_duration_seconds_sum{code="503",method="POST",route_name="app-beta",route_namespace="staging"} 12
		interceptor_request_duration_seconds_count{code="503",method="POST",route_name="app-beta",route_namespace="staging"} 1
	`
	if err := testutil.CollectAndCompare(registry, strings.NewReader(expected), "interceptor_request_duration_seconds"); err != nil {
		t.Fatalf("unexpected metrics output:\n%v", err)
	}
}

func TestPrometheus_ConcurrencyMetrics(t *testing.T) {
	registry, instruments := testRegistry(t)

	instruments.RecordPendingRequest("my-route", "my-ns", 5)

	expected := `
		# HELP interceptor_request_concurrency Concurrent requests at the interceptor proxy
		# TYPE interceptor_request_concurrency gauge
		interceptor_request_concurrency{route_name="my-route",route_namespace="my-ns"} 5
	`
	if err := testutil.CollectAndCompare(registry, strings.NewReader(expected), "interceptor_request_concurrency"); err != nil {
		t.Fatalf("unexpected metrics output:\n%v", err)
	}
}
