package config

import (
	"time"

	"github.com/caarlos0/env/v11"
	"github.com/go-logr/logr"
)

// Timeouts is the configuration for request handling and connection timeouts.
type Timeouts struct {
	// Request is the total wall-clock deadline from request arrival to response completion.
	// When 0 (the default), there is no total request deadline.
	Request time.Duration `env:"KEDA_HTTP_REQUEST_TIMEOUT" envDefault:"0s"`
	// ResponseHeader is how long to wait between when the HTTP request
	// is sent to the backing app and when response headers need to arrive.
	// Defaults to 300s as a safety net against hung backends. Set to 0 to disable.
	ResponseHeader time.Duration `env:"KEDA_HTTP_RESPONSE_HEADER_TIMEOUT" envDefault:"300s"`
	// Readiness is how long to wait for the backing workload
	// to have 1 or more replicas before connecting and sending the HTTP request.
	// When 0 (the default), the readiness wait is bounded only by the request
	// timeout, giving the full request budget to cold starts. When a fallback
	// service is configured and this is 0, a 30s default is applied.
	Readiness time.Duration `env:"KEDA_HTTP_READINESS_TIMEOUT" envDefault:"0s"`
	// Connect is the per-attempt TCP dial timeout (net.Dialer.Timeout).
	// Bounded by the request context deadline.
	Connect time.Duration `env:"KEDA_HTTP_CONNECT_TIMEOUT" envDefault:"500ms"`

	// MaxIdleConns is the max number of idle connections to keep in the
	// interceptor's internal connection pool across all backend services.
	// Increase this if you proxy to many unique backend services.
	MaxIdleConns int `env:"KEDA_HTTP_MAX_IDLE_CONNS" envDefault:"1000"`
	// MaxIdleConnsPerHost is the max number of idle connections to keep per backend service.
	// Increase this if you observe many new connection establishments under load.
	MaxIdleConnsPerHost int `env:"KEDA_HTTP_MAX_IDLE_CONNS_PER_HOST" envDefault:"200"`
	// ForceHTTP2 toggles whether to try to force HTTP2 for all requests.
	ForceHTTP2 bool `env:"KEDA_HTTP_FORCE_HTTP2" envDefault:"false"`
}

// deprecatedTimeouts holds deprecated env vars that take precedence when set.
type deprecatedTimeouts struct {
	// ResponseHeader is how long to wait between when the HTTP request
	// is sent to the backing app and when response headers need to arrive.
	ResponseHeader time.Duration `env:"KEDA_RESPONSE_HEADER_TIMEOUT"`
	// WorkloadReplicas is how long to wait for the backing workload
	// to have 1 or more replicas before connecting and sending the HTTP request.
	WorkloadReplicas time.Duration `env:"KEDA_CONDITION_WAIT_TIMEOUT"`
}

// MustParseTimeouts parses timeout configuration from environment variables.
// Deprecated env vars take precedence over new ones when set, to preserve
// existing behavior for users who haven't migrated yet.
func MustParseTimeouts(log logr.Logger) Timeouts {
	cfg := env.Must(env.ParseAs[Timeouts]())

	deprecated := env.Must(env.ParseAs[deprecatedTimeouts]())

	if deprecated.WorkloadReplicas > 0 {
		log.Info("WARNING: KEDA_CONDITION_WAIT_TIMEOUT is deprecated, use KEDA_HTTP_READINESS_TIMEOUT instead")
		cfg.Readiness = deprecated.WorkloadReplicas
	}

	if deprecated.ResponseHeader > 0 {
		log.Info("WARNING: KEDA_RESPONSE_HEADER_TIMEOUT is deprecated, use KEDA_HTTP_RESPONSE_HEADER_TIMEOUT instead")
		cfg.ResponseHeader = deprecated.ResponseHeader
	}

	return cfg
}
