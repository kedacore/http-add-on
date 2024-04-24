package config

import (
	"github.com/kelseyhightower/envconfig"
)

// Metrics is the configuration for configuring metrics in the interceptor.
type Metrics struct {
	// Sets whether or not to enable the Prometheus metrics exporter
	OtelPrometheusExporterEnabled bool `envconfig:"KEDA_HTTP_OTEL_PROM_EXPORTER_ENABLED" default:"true"`
	// Sets the port which the Prometheus compatible metrics endpoint should be served on
	OtelPrometheusExporterPort int `envconfig:"KEDA_HTTP_OTEL_PROM_EXPORTER_PORT" default:"2223"`
	// Sets whether or not to enable the OTEL metrics exporter
	OtelHTTPExporterEnabled bool `envconfig:"KEDA_HTTP_OTEL_HTTP_EXPORTER_ENABLED" default:"false"`
	// Sets the HTTP endpoint where metrics should be sent to
	OtelHTTPCollectorEndpoint string `envconfig:"KEDA_HTTP_OTEL_HTTP_COLLECTOR_ENDPOINT" default:"localhost:4318"`
	// Sets the OTLP headers required by the otel exporter
	OtelHTTPHeaders string `envconfig:"KEDA_HTTP_OTEL_HTTP_HEADERS" default:""`
	// Set the connection to the otel HTTP collector endpoint to use HTTP rather than HTTPS
	OtelHTTPCollectorInsecure bool `envconfig:"KEDA_HTTP_OTEL_HTTP_COLLECTOR_INSECURE" default:"false"`
	// Set the interval in seconds to export otel metrics to the configured collector endpoint
	OtelMetricExportInterval int `envconfig:"KEDA_HTTP_OTEL_METRIC_EXPORT_INTERVAL" default:"30"`
}

// Parse parses standard configs using envconfig and returns a pointer to the
// newly created config. Returns nil and a non-nil error if parsing failed
func MustParseMetrics() *Metrics {
	ret := new(Metrics)
	envconfig.MustProcess("", ret)
	return ret
}
