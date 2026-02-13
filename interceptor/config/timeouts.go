package config

import (
	"time"

	"github.com/kelseyhightower/envconfig"
)

// Timeouts is the configuration for connection and HTTP timeouts
type Timeouts struct {
	// Connect is the per-attempt TCP dial timeout (net.Dialer.Timeout)
	Connect time.Duration `envconfig:"KEDA_HTTP_CONNECT_TIMEOUT" default:"500ms"`
	// KeepAlive is the interval between keepalive probes
	KeepAlive time.Duration `envconfig:"KEDA_HTTP_KEEP_ALIVE" default:"1s"`
	// ResponseHeaderTimeout is how long to wait between when the HTTP request
	// is sent to the backing app and when response headers need to arrive
	ResponseHeader time.Duration `envconfig:"KEDA_RESPONSE_HEADER_TIMEOUT" default:"500ms"`
	// WorkloadReplicas is how long to wait for the backing workload
	// to have 1 or more replicas before connecting and sending the HTTP request.
	WorkloadReplicas time.Duration `envconfig:"KEDA_CONDITION_WAIT_TIMEOUT" default:"20s"`
	// ForceHTTP2 toggles whether to try to force HTTP2 for all requests
	ForceHTTP2 bool `envconfig:"KEDA_HTTP_FORCE_HTTP2" default:"false"`
	// MaxIdleConns is the max number of idle connections to keep in the
	// interceptor's internal connection pool across all backend services.
	// Increase this if you proxy to many unique backend services.
	MaxIdleConns int `envconfig:"KEDA_HTTP_MAX_IDLE_CONNS" default:"100"`
	// MaxIdleConnsPerHost is the max number of idle connections to keep per backend service.
	// Increase this if you observe many new connection establishments under load.
	MaxIdleConnsPerHost int `envconfig:"KEDA_HTTP_MAX_IDLE_CONNS_PER_HOST" default:"20"`
	// IdleConnTimeout is the timeout after which a connection in the interceptor's
	// internal connection pool will be closed
	IdleConnTimeout time.Duration `envconfig:"KEDA_HTTP_IDLE_CONN_TIMEOUT" default:"90s"`
	// TLSHandshakeTimeout is the max amount of time the interceptor will
	// wait to establish a TLS connection
	TLSHandshakeTimeout time.Duration `envconfig:"KEDA_HTTP_TLS_HANDSHAKE_TIMEOUT" default:"10s"`
	// ExpectContinueTimeout is the max amount of time the interceptor will wait
	// for a 100 Continue response from the backend after sending request headers
	// with Expect: 100-continue
	ExpectContinueTimeout time.Duration `envconfig:"KEDA_HTTP_EXPECT_CONTINUE_TIMEOUT" default:"1s"`
	// DialRetryTimeout caps the total time spent retrying failed dial attempts.
	DialRetryTimeout time.Duration `envconfig:"KEDA_HTTP_DIAL_RETRY_TIMEOUT" default:"15s"`
}

// MustParseTimeouts parses standard configs using envconfig and returns the
// newly created config. It panics if parsing fails.
func MustParseTimeouts() Timeouts {
	var ret Timeouts
	envconfig.MustProcess("", &ret)
	return ret
}
