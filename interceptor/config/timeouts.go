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
	// DeploymentReplicas is how long to wait for the backing deployment
	// to have 1 or more replicas before connecting and sending the HTTP request.
	DeploymentReplicas time.Duration `envconfig:"KEDA_CONDITION_WAIT_TIMEOUT" default:"1500ms"`
	// ForceHTTP2 toggles whether to try to force HTTP2 for all requests
	ForceHTTP2 bool `envconfig:"KEDA_HTTP_FORCE_HTTP2" default:"false"`
	// MaxIdleConns is the max number of connections that can be idle in the
	// interceptor's internal connection pool
	MaxIdleConns int `envconfig:"KEDA_HTTP_MAX_IDLE_CONNS" default:"100"`
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

// Backoff returns a wait.Backoff based on the timeouts in t
func (t *Timeouts) Backoff(factor, jitter float64, steps int) wait.Backoff {
	return wait.Backoff{
		Duration: t.Connect,
		Factor:   factor,
		Jitter:   jitter,
		Steps:    steps,
	}
}

// DefaultBackoff calls t.Backoff with reasonable defaults and returns
// the result
func (t *Timeouts) DefaultBackoff() wait.Backoff {
	return t.Backoff(2, 0.5, 5)
}

// Parse parses standard configs using envconfig and returns a pointer to the
// newly created config. Returns nil and a non-nil error if parsing failed
func MustParseTimeouts() *Timeouts {
	ret := new(Timeouts)
	envconfig.MustProcess("", ret)
	return ret
}
