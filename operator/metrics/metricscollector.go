package metrics

import (
	"go.opentelemetry.io/otel/exporters/prometheus"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	"github.com/kedacore/http-add-on/operator/controllers/http/config"
)

var (
	collectors []Collector
)

const meterName = "keda-http-add-on-operator"

type Collector interface {
	RecordHTTPScaledObjectCount(namespace string)
	RecordDeleteHTTPScaledObjectCount(namespace string)
}

func NewMetricsCollectors(metricsConfig *config.Metrics) {
	if metricsConfig.OtelPrometheusExporterEnabled {
		options := prometheus.WithRegisterer(ctrlmetrics.Registry)
		promometrics := NewPrometheusMetrics(options)
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

func RecordDeleteHTTPScaledObjectCount(namespace string) {
	for _, collector := range collectors {
		collector.RecordDeleteHTTPScaledObjectCount(namespace)
	}
}
