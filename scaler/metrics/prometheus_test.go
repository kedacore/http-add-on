package metrics

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	promexporter "go.opentelemetry.io/otel/exporters/prometheus"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"

	"github.com/kedacore/http-add-on/pkg/observability"
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

	provider, err := observability.NewMeterProvider(
		ServiceName,
		observability.MetricsConfig{OtelPrometheusExporterEnabled: false},
		sdkmetric.WithReader(exporter),
	)
	if err != nil {
		t.Fatalf("observability.NewMeterProvider() error: %v", err)
	}
	t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })

	instruments, err := NewInstruments(provider)
	if err != nil {
		t.Fatalf("NewInstruments() error: %v", err)
	}

	return registry, instruments
}

func TestPrometheus_FetchDuration(t *testing.T) {
	registry, instruments := testRegistry(t)

	instruments.RecordFetch(50*time.Millisecond, 3, 0, nil)
	instruments.RecordFetch(200*time.Millisecond, 3, 0, nil)

	expected := `
		# HELP scaler_pinger_fetch_duration_seconds Duration of a queue pinger fetch cycle across all interceptor pods
		# TYPE scaler_pinger_fetch_duration_seconds histogram
		scaler_pinger_fetch_duration_seconds_bucket{le="0.005"} 0
		scaler_pinger_fetch_duration_seconds_bucket{le="0.01"} 0
		scaler_pinger_fetch_duration_seconds_bucket{le="0.025"} 0
		scaler_pinger_fetch_duration_seconds_bucket{le="0.05"} 1
		scaler_pinger_fetch_duration_seconds_bucket{le="0.075"} 1
		scaler_pinger_fetch_duration_seconds_bucket{le="0.1"} 1
		scaler_pinger_fetch_duration_seconds_bucket{le="0.25"} 2
		scaler_pinger_fetch_duration_seconds_bucket{le="0.5"} 2
		scaler_pinger_fetch_duration_seconds_bucket{le="0.75"} 2
		scaler_pinger_fetch_duration_seconds_bucket{le="1"} 2
		scaler_pinger_fetch_duration_seconds_bucket{le="2.5"} 2
		scaler_pinger_fetch_duration_seconds_bucket{le="5"} 2
		scaler_pinger_fetch_duration_seconds_bucket{le="+Inf"} 2
		scaler_pinger_fetch_duration_seconds_sum 0.25
		scaler_pinger_fetch_duration_seconds_count 2
	`
	if err := testutil.CollectAndCompare(registry, strings.NewReader(expected), "scaler_pinger_fetch_duration_seconds"); err != nil {
		t.Fatalf("unexpected metrics output:\n%v", err)
	}
}

func TestPrometheus_FetchErrors(t *testing.T) {
	registry, instruments := testRegistry(t)

	instruments.RecordFetch(10*time.Millisecond, 2, 2, errors.New("connection refused"))
	instruments.RecordFetch(10*time.Millisecond, 2, 0, nil)
	instruments.RecordFetch(10*time.Millisecond, 0, 0, errors.New("no endpoints"))

	expected := `
		# HELP scaler_pinger_fetch_errors_total Total failed queue pinger fetch cycles
		# TYPE scaler_pinger_fetch_errors_total counter
		scaler_pinger_fetch_errors_total 2
	`
	if err := testutil.CollectAndCompare(registry, strings.NewReader(expected), "scaler_pinger_fetch_errors_total"); err != nil {
		t.Fatalf("unexpected metrics output:\n%v", err)
	}
}

func TestPrometheus_UnreachablePods(t *testing.T) {
	registry, instruments := testRegistry(t)

	instruments.RecordFetch(10*time.Millisecond, 3, 1, nil)
	instruments.RecordFetch(10*time.Millisecond, 3, 0, nil)
	instruments.RecordFetch(10*time.Millisecond, 3, 2, nil)

	expected := `
		# HELP scaler_pinger_unreachable_pods Number of interceptor pods that were unreachable in the last fetch cycle
		# TYPE scaler_pinger_unreachable_pods gauge
		scaler_pinger_unreachable_pods 2
	`
	if err := testutil.CollectAndCompare(registry, strings.NewReader(expected), "scaler_pinger_unreachable_pods"); err != nil {
		t.Fatalf("unexpected metrics output:\n%v", err)
	}
}

func TestPrometheus_Endpoints(t *testing.T) {
	registry, instruments := testRegistry(t)

	instruments.RecordFetch(10*time.Millisecond, 5, 0, nil)

	expected := `
		# HELP scaler_pinger_endpoints Number of interceptor endpoints the scaler is polling
		# TYPE scaler_pinger_endpoints gauge
		scaler_pinger_endpoints 5
	`
	if err := testutil.CollectAndCompare(registry, strings.NewReader(expected), "scaler_pinger_endpoints"); err != nil {
		t.Fatalf("unexpected metrics output:\n%v", err)
	}
}
