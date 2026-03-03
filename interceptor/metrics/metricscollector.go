package metrics

import (
	"github.com/kedacore/http-add-on/interceptor/config"
)

var collectors []Collector

const meterName = "keda-interceptor-proxy"

type Collector interface {
	RecordRequestCount(method string, path string, responseCode int, host string)
	RecordPendingRequestCount(host string, value int64)
}

func NewMetricsCollectors(metricsConfig config.Metrics) {
	if metricsConfig.OtelPrometheusExporterEnabled {
		promometrics := NewPrometheusMetrics()
		collectors = append(collectors, promometrics)
	}

	if metricsConfig.OtelHTTPExporterEnabled {
		otelhttpmetrics := NewOtelMetrics()
		collectors = append(collectors, otelhttpmetrics)
	}
}

func RecordRequestCount(method string, path string, responseCode int, host string) {
	for _, collector := range collectors {
		collector.RecordRequestCount(method, path, responseCode, host)
	}
}

func RecordPendingRequestCount(host string, value int64) {
	for _, collector := range collectors {
		collector.RecordPendingRequestCount(host, value)
	}
}
