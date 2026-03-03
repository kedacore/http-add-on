package metrics

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

var (
	testOtel   *OtelMetrics
	testReader metric.Reader
)

func init() {
	testReader = metric.NewManualReader()
	options := metric.WithReader(testReader)
	testOtel = NewOtelMetrics(options)
}

func TestRequestCounter(t *testing.T) {
	testOtel.RecordRequestCount("GET", "/test", 200, "test-host-1")
	got := metricdata.ResourceMetrics{}
	err := testReader.Collect(context.Background(), &got)

	require.NoError(t, err)
	scopeMetrics := got.ScopeMetrics[0]
	assert.NotEmpty(t, scopeMetrics.Metrics)

	metricInfo := retrieveMetric(scopeMetrics.Metrics, "interceptor_request_count")
	data := metricInfo.Data.(metricdata.Sum[int64]).DataPoints[0]
	assert.Equal(t, int64(1), data.Value)
}

func TestPendingRequestCounter(t *testing.T) {
	testOtel.RecordPendingRequestCount("test-host", 5)
	got := metricdata.ResourceMetrics{}
	err := testReader.Collect(context.Background(), &got)

	require.NoError(t, err)
	scopeMetrics := got.ScopeMetrics[0]
	assert.NotEmpty(t, scopeMetrics.Metrics)

	metricInfo := retrieveMetric(scopeMetrics.Metrics, "interceptor_pending_request_count")
	data := metricInfo.Data.(metricdata.Sum[int64]).DataPoints[0]
	assert.Equal(t, int64(5), data.Value)
}

func retrieveMetric(metrics []metricdata.Metrics, metricname string) *metricdata.Metrics {
	for _, m := range metrics {
		if m.Name == metricname {
			return &m
		}
	}
	return nil
}
