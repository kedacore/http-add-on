package config

import (
	"github.com/kelseyhightower/envconfig"
)

// Metrics is the configuration for configuring metrics in the interceptor.
type Metrics struct {
	// Sets whether or not to enable the Prometheus metrics exporter
	OtelPrometheusExporterEnabled bool `envconfig:"OTEL_PROM_EXPORTER_ENABLED" default:"true"`
	// Sets the port which the Prometheus compatible metrics endpoint should be served on
	OtelPrometheusExporterPort int `envconfig:"OTEL_PROM_EXPORTER_PORT" default:"2223"`
	// Sets whether or not to enable the OTEL metrics exporter
	OtelHTTPExporterEnabled bool `envconfig:"OTEL_EXPORTER_OTLP_METRICS_ENABLED" default:"false"`
}

// Parse parses standard configs using envconfig and returns a pointer to the
// newly created config. Returns nil and a non-nil error if parsing failed
func MustParseMetrics() *Metrics {
	ret := new(Metrics)
	envconfig.MustProcess("", ret)
	return ret
}
