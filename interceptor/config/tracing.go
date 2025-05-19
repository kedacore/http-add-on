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

// Parse parses standard configs using envconfig and returns a pointer to the
// newly created config. Returns nil and a non-nil error if parsing failed
func MustParseTracing() *Tracing {
	ret := new(Tracing)
	envconfig.MustProcess("", ret)
	return ret
}
