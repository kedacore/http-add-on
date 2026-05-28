package observability

import (
	"github.com/caarlos0/env/v11"
)

// MetricsConfig is the configuration for OTEL metrics export.
type MetricsConfig struct {
	OtelPrometheusExporterEnabled bool `env:"OTEL_PROM_EXPORTER_ENABLED" envDefault:"true"`
	OtelPrometheusExporterPort    int  `env:"OTEL_PROM_EXPORTER_PORT" envDefault:"2223"`
	OtelHTTPExporterEnabled       bool `env:"OTEL_EXPORTER_OTLP_METRICS_ENABLED" envDefault:"false"`
}

// TracingConfig is the configuration for OTEL tracing.
type TracingConfig struct {
	Enabled  bool   `env:"OTEL_EXPORTER_OTLP_TRACES_ENABLED" envDefault:"false"`
	Exporter string `env:"OTEL_EXPORTER_OTLP_TRACES_PROTOCOL" envDefault:"console"`
}

// MustParseMetricsConfig parses metrics config from environment variables.
func MustParseMetricsConfig() MetricsConfig {
	return env.Must(env.ParseAs[MetricsConfig]())
}

// MustParseTracingConfig parses tracing config from environment variables.
func MustParseTracingConfig() TracingConfig {
	return env.Must(env.ParseAs[TracingConfig]())
}
