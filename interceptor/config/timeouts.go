package config

import (
	"time"

	"github.com/caarlos0/env/v11"
)

// Timeouts is the configuration for connection and HTTP timeouts
type Timeouts struct {
	// Connect is the per-attempt TCP dial timeout (net.Dialer.Timeout)
	Connect time.Duration `env:"KEDA_HTTP_CONNECT_TIMEOUT" envDefault:"500ms"`
	// KeepAlive is the interval between keepalive probes
	KeepAlive time.Duration `env:"KEDA_HTTP_KEEP_ALIVE" envDefault:"1s"`
	// ResponseHeaderTimeout is how long to wait between when the HTTP request
	// is sent to the backing app and when response headers need to arrive
	ResponseHeader time.Duration `env:"KEDA_RESPONSE_HEADER_TIMEOUT" envDefault:"500ms"`
	// WorkloadReplicas is how long to wait for the backing workload
	// to have 1 or more replicas before connecting and sending the HTTP request.
	WorkloadReplicas time.Duration `env:"KEDA_CONDITION_WAIT_TIMEOUT" envDefault:"20s"`
	// ForceHTTP2 toggles whether to try to force HTTP2 for all requests
	ForceHTTP2 bool `env:"KEDA_HTTP_FORCE_HTTP2" envDefault:"false"`
	// DisableKeepAlives disables HTTP keep-alives for requests from interceptor to backend services.
	DisableKeepAlives bool `env:"KEDA_HTTP_DISABLE_KEEP_ALIVES" envDefault:"false"`
	// MaxIdleConns is the max number of idle connections to keep in the
	// interceptor's internal connection pool across all backend services.
	// Increase this if you proxy to many unique backend services.
	MaxIdleConns int `env:"KEDA_HTTP_MAX_IDLE_CONNS" envDefault:"1000"`
	// MaxIdleConnsPerHost is the max number of idle connections to keep per backend service.
	// Increase this if you observe many new connection establishments under load.
	MaxIdleConnsPerHost int `env:"KEDA_HTTP_MAX_IDLE_CONNS_PER_HOST" envDefault:"200"`
	// IdleConnTimeout is the timeout after which a connection in the interceptor's
	// internal connection pool will be closed
	IdleConnTimeout time.Duration `env:"KEDA_HTTP_IDLE_CONN_TIMEOUT" envDefault:"90s"`
	// TLSHandshakeTimeout is the max amount of time the interceptor will
	// wait to establish a TLS connection
	TLSHandshakeTimeout time.Duration `env:"KEDA_HTTP_TLS_HANDSHAKE_TIMEOUT" envDefault:"10s"`
	// ExpectContinueTimeout is the max amount of time the interceptor will wait
	// for a 100 Continue response from the backend after sending request headers
	// with Expect: 100-continue
	ExpectContinueTimeout time.Duration `env:"KEDA_HTTP_EXPECT_CONTINUE_TIMEOUT" envDefault:"1s"`
	// DialRetryTimeout caps the total time spent retrying failed dial attempts.
	DialRetryTimeout time.Duration `env:"KEDA_HTTP_DIAL_RETRY_TIMEOUT" envDefault:"15s"`
}

// MustParseTimeouts parses standard configs and returns the
// newly created config. It panics if parsing fails.
func MustParseTimeouts() Timeouts {
	return env.Must(env.ParseAs[Timeouts]())
}
