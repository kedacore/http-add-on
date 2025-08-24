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
	# HELP keda_http_scaled_object_total a counter of http_scaled_objects processed by the operator
	# TYPE keda_http_scaled_object_total gauge
	keda_http_scaled_object_total{namespace="test-namespace",otel_scope_name="keda-http-add-on-operator",otel_scope_version=""} 0
	keda_http_scaled_object_total{namespace="other-test-namespace",otel_scope_name="keda-http-add-on-operator",otel_scope_version=""} 1
	# HELP otel_scope_info Instrumentation Scope metadata
	# TYPE otel_scope_info gauge
	otel_scope_info{otel_scope_name="keda-http-add-on-operator",otel_scope_version=""} 1
	# HELP target_info Target metadata
	# TYPE target_info gauge
	target_info{"service.name"="http-add-on-operator","service.version"="main"} 1
	`
	expectedOutputReader := strings.NewReader(expectedOutput)
	testPrometheus.RecordHTTPScaledObjectCount("test-namespace")
	testPrometheus.RecordDeleteHTTPScaledObjectCount("test-namespace")
	testPrometheus.RecordHTTPScaledObjectCount("other-test-namespace")
	err := testutil.CollectAndCompare(testRegistry, expectedOutputReader)
	assert.Nil(t, err)
}
