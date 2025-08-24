package metrics

import (
	"github.com/kedacore/http-add-on/operator/controllers/http/config"
)

var (
	collectors []Collector
)

const meterName = "keda-http-add-on-operator"

type Collector interface {
	RecordHTTPScaledObjectCount(namespace string)
}

func NewMetricsCollectors(metricsConfig *config.Metrics) {
	if metricsConfig.OtelPrometheusExporterEnabled {
		promometrics := NewPrometheusMetrics()
		collectors = append(collectors, promometrics)
	}

	if metricsConfig.OtelHTTPExporterEnabled {
		otelhttpmetrics := NewOtelMetrics()
		collectors = append(collectors, otelhttpmetrics)
	}
}

func RecordHTTPScaledObjectCount(namespace string) {
	for _, collector := range collectors {
		collector.RecordHTTPScaledObjectCount(namespace)
	}
}
