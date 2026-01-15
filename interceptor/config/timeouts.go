package config

import (
	"time"

	"github.com/kelseyhightower/envconfig"
	"k8s.io/apimachinery/pkg/util/wait"
)

// Timeouts is the configuration for connection and HTTP timeouts
type Timeouts struct {
	// Connect is the connection timeout
	Connect time.Duration `envconfig:"KEDA_HTTP_CONNECT_TIMEOUT" default:"500ms"`
	// KeepAlive is the interval between keepalive probes
	KeepAlive time.Duration `envconfig:"KEDA_HTTP_KEEP_ALIVE" default:"1s"`
	// ResponseHeaderTimeout is how long to wait between when the HTTP request
	// is sent to the backing app and when response headers need to arrive
	ResponseHeader time.Duration `envconfig:"KEDA_RESPONSE_HEADER_TIMEOUT" default:"500ms"`
	// WorkloadReplicas is how long to wait for the backing workload
	// to have 1 or more replicas before connecting and sending the HTTP request.
	WorkloadReplicas time.Duration `envconfig:"KEDA_CONDITION_WAIT_TIMEOUT" default:"1500ms"`
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
	// after sending request headers if the server returned an Expect: 100-continue
	// header
	ExpectContinueTimeout time.Duration `envconfig:"KEDA_HTTP_EXPECT_CONTINUE_TIMEOUT" default:"1s"`
}

// DefaultBackoff returns backoff config optimized for cold start pod availability polling.
// Based on https://github.com/knative/networking/blob/main/test/conformance/ingress/util.go#L70
func (t Timeouts) DefaultBackoff() wait.Backoff {
	return wait.Backoff{
		Cap:      1 * time.Second, // max sleep time per step, e.g. attempt a connection at least every second
		Duration: 50 * time.Millisecond,
		Factor:   1.4,
		Jitter:   0.1, // adds up to +10% randomness
		Steps:    10,
	}
}

// MustParseTimeouts parses standard configs using envconfig and returns a pointer to the
// newly created config. It panics if parsing fails.
func MustParseTimeouts() *Timeouts {
	ret := new(Timeouts)
	envconfig.MustProcess("", ret)
	return ret
}
