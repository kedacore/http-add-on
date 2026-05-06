package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/kedacore/http-add-on/interceptor/metrics"
)

func TestMetrics_RecordsRequestWithRouteInfo(t *testing.T) {
	instruments, reader := testInstruments(t)

	// Simulate routing by populating routeInfo
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ri := routeInfoFromContext(r.Context()); ri != nil {
			ri.Name = "test-route"
			ri.Namespace = "test-ns"
		}
		w.WriteHeader(http.StatusOK)
	})

	handler := NewMetrics(next, instruments)

	req := httptest.NewRequest("GET", "/some/path", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &rm); err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	m := requireMetric(t, rm, metrics.MetricRequestCount)

	sum := m.Data.(metricdata.Sum[int64])
	if got := len(sum.DataPoints); got != 1 {
		t.Fatalf("expected 1 data point, got %d", got)
	}

	dp := sum.DataPoints[0]
	assertStringAttr(t, dp.Attributes, metrics.AttrMethod, http.MethodGet)
	assertIntAttr(t, dp.Attributes, metrics.AttrCode, int64(http.StatusOK))
	assertStringAttr(t, dp.Attributes, metrics.AttrRouteName, "test-route")
	assertStringAttr(t, dp.Attributes, metrics.AttrRouteNamespace, "test-ns")

	requireMetric(t, rm, metrics.MetricRequestDuration)
}

func TestMetrics_UnmatchedRoute(t *testing.T) {
	instruments, reader := testInstruments(t)

	// Do not populate routeInfo — simulates unmatched route
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	handler := NewMetrics(next, instruments)

	req := httptest.NewRequest("GET", "/unknown", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &rm); err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	m := requireMetric(t, rm, metrics.MetricRequestCount)

	dp := m.Data.(metricdata.Sum[int64]).DataPoints[0]
	assertStringAttr(t, dp.Attributes, metrics.AttrRouteName, "")
}

func testInstruments(t *testing.T) (*metrics.Instruments, sdkmetric.Reader) {
	t.Helper()

	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })

	instruments, err := metrics.NewInstruments(provider)
	if err != nil {
		t.Fatalf("NewInstruments() error: %v", err)
	}

	return instruments, reader
}

func assertStringAttr(t *testing.T, attrs attribute.Set, key, want string) {
	t.Helper()
	v, _ := attrs.Value(attribute.Key(key))
	if got := v.AsString(); got != want {
		t.Fatalf("attribute %s: got %q, want %q", key, got, want)
	}
}

func assertIntAttr(t *testing.T, attrs attribute.Set, key string, want int64) {
	t.Helper()
	v, _ := attrs.Value(attribute.Key(key))
	if got := v.AsInt64(); got != want {
		t.Fatalf("attribute %s: got %d, want %d", key, got, want)
	}
}
