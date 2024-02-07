package metrics

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/kedacore/http-add-on/interceptor/config"
)

var (
	testOtel   *OtelMetrics
	testReader metric.Reader
)

func init() {
	testReader = metric.NewManualReader()
	options := metric.WithReader(testReader)
	metricsCfg := config.Metrics{}
	testOtel = NewOtelMetrics(&metricsCfg, options)
}

func TestRequestCounter(t *testing.T) {
	testOtel.RecordRequestCount("GET", "/test", 200, "test-host-1")
	got := metricdata.ResourceMetrics{}
	err := testReader.Collect(context.Background(), &got)

	assert.Nil(t, err)
	scopeMetrics := got.ScopeMetrics[0]
	assert.NotEqual(t, len(scopeMetrics.Metrics), 0)

	metricInfo := retrieveMetric(scopeMetrics.Metrics, "interceptor_request_count")
	data := metricInfo.Data.(metricdata.Sum[int64]).DataPoints[0]
	assert.Equal(t, data.Value, int64(1))
}

func TestPendingRequestGuage(t *testing.T) {
	testOtel.RecordPendingRequestCount("test-host", 5)
	got := metricdata.ResourceMetrics{}
	err := testReader.Collect(context.Background(), &got)

	assert.Nil(t, err)
	scopeMetrics := got.ScopeMetrics[0]
	assert.NotEqual(t, len(scopeMetrics.Metrics), 0)

	metricInfo := retrieveMetric(scopeMetrics.Metrics, "interceptor_pending_request_count")
	data := metricInfo.Data.(metricdata.Gauge[int64]).DataPoints[0]
	assert.Equal(t, data.Value, int64(5))
}

func retrieveMetric(metrics []metricdata.Metrics, metricname string) *metricdata.Metrics {
	for _, m := range metrics {
		if m.Name == metricname {
			return &m
		}
	}
	return nil
}
