package metrics

import (
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	promexporter "go.opentelemetry.io/otel/exporters/prometheus"
)

func TestPromRequestCountMetric(t *testing.T) {
	testRegistry := prometheus.NewRegistry()
	options := []promexporter.Option{promexporter.WithRegisterer(testRegistry)}
	testPrometheus := NewPrometheusMetrics(options...)
	expectedOutput := `
	# HELP interceptor_request_count_total a counter of requests processed by the interceptor proxy
	# TYPE interceptor_request_count_total counter
    interceptor_request_count_total{code="500",host="test-host",method="post",otel_scope_name="keda-interceptor-proxy",otel_scope_schema_url="",otel_scope_version="",path="/test"} 1
    interceptor_request_count_total{code="200",host="test-host",method="post",otel_scope_name="keda-interceptor-proxy",otel_scope_schema_url="",otel_scope_version="",path="/test"} 1
	# HELP target_info Target metadata
	# TYPE target_info gauge
	target_info{service_name="interceptor-proxy",service_version="HEAD"} 1
	`
	expectedOutputReader := strings.NewReader(expectedOutput)
	testPrometheus.RecordRequestCount("post", "/test", 500, "test-host")
	testPrometheus.RecordRequestCount("post", "/test", 200, "test-host")
	err := testutil.CollectAndCompare(testRegistry, expectedOutputReader)
	assert.Nil(t, err)
}

func TestPromPendingRequestCountMetric(t *testing.T) {
	testRegistry := prometheus.NewRegistry()
	options := []promexporter.Option{promexporter.WithRegisterer(testRegistry)}
	testPrometheus := NewPrometheusMetrics(options...)
	expectedOutput := `
	# HELP interceptor_pending_request_count a count of requests pending forwarding by the interceptor proxy
	# TYPE interceptor_pending_request_count gauge
	interceptor_pending_request_count{host="test-host",otel_scope_name="keda-interceptor-proxy",otel_scope_schema_url="",otel_scope_version=""} 10
	# HELP target_info Target metadata
	# TYPE target_info gauge
	target_info{service_name="interceptor-proxy",service_version="HEAD"} 1
	`
	expectedOutputReader := strings.NewReader(expectedOutput)
	testPrometheus.RecordPendingRequestCount("test-host", 10)
	err := testutil.CollectAndCompare(testRegistry, expectedOutputReader)
	assert.Nil(t, err)
}
