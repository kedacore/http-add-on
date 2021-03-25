package config

import (
	"time"

	"github.com/kelseyhightower/envconfig"
	"k8s.io/apimachinery/pkg/util/wait"
)

// Timeouts is the configuration for connection and HTTP timeouts
type Timeouts struct {
	// connection timeout
	Connect time.Duration `envconfig:"KEDA_HTTP_CONNECT_TIMEOUT"`
	// interval between keepalive probes
	KeepAlive time.Duration `envconfig:"KEDA_HTTP_KEEP_ALIVE"`
	// timeout between when HTTP request is sent and response headers need
	// to show up
	ResponseHeader time.Duration `envconfig:"KEDA_RESPONSE_HEADER_TIMEOUT"`
	// time to wait for condition before connecting and sending request.
	// most commonly, this is how long to wait until the origin deployment
	// has >= 1 replica
	WaitFunc time.Duration `envconfig:"KEDA_CONDITION_WAIT_TIMEOUT"`
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

// ParseTimeouts parses a Timeouts struct using envconfig, and returns nil and a
// non-nil error if parsing failed
func ParseTimeouts() (*Timeouts, error) {
	ret := new(Timeouts)
	if err := envconfig.Process("", ret); err != nil {
		return nil, err
	}
	return ret, nil
}
