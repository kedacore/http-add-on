package config

import (
	"github.com/kedacore/http-add-on/pkg/observability"
)

// Tracing is an alias for the shared observability tracing config.
// Kept to avoid refactoring all existing interceptor code that references config.Tracing.
type Tracing = observability.TracingConfig
