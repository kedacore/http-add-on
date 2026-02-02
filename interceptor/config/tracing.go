package config

import (
	"github.com/kelseyhightower/envconfig"
)

// Tracing is the configuration for configuring tracing through the interceptor.
type Tracing struct {
	// States whether tracing should be enabled, False by default
	Enabled bool `envconfig:"OTEL_EXPORTER_OTLP_TRACES_ENABLED" default:"false"`
	// Sets what tracing export to use, must be one of: console,http/protobuf, grpc
	Exporter string `envconfig:"OTEL_EXPORTER_OTLP_TRACES_PROTOCOL" default:"console"`
}

// MustParseTracing parses standard configs using envconfig and returns the
// newly created config. It panics if parsing fails.
func MustParseTracing() Tracing {
	var ret Tracing
	envconfig.MustProcess("", &ret)
	return ret
}
