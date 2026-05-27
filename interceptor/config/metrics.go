package config

import (
	"github.com/kedacore/http-add-on/pkg/observability"
)

// Metrics is an alias for the shared observability metrics config.
// Kept to avoid refactoring all existing interceptor code that references config.Metrics.
type Metrics = observability.MetricsConfig
