package config

import (
	"github.com/caarlos0/env/v11"
)

// Tracing is the configuration for configuring tracing through the interceptor.
type Tracing struct {
	// States whether tracing should be enabled, False by default
	Enabled bool `env:"OTEL_EXPORTER_OTLP_TRACES_ENABLED" envDefault:"false"`
	// Sets what tracing export to use, must be one of: console, http/protobuf, grpc
	Exporter string `env:"OTEL_EXPORTER_OTLP_TRACES_PROTOCOL" envDefault:"console"`
}

// MustParseTracing parses standard configs and returns the
// newly created config. It panics if parsing fails.
func MustParseTracing() Tracing {
	return env.Must(env.ParseAs[Tracing]())
}
