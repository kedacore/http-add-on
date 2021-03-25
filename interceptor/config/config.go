package config

import (
	"time"

	"github.com/kelseyhightower/envconfig"
	"k8s.io/apimachinery/pkg/util/wait"
)

// Timeouts is the configuration for connection and HTTP timeouts
type Timeouts struct {
	Connect        time.Duration `envconfig:"KEDA_HTTP_CONNECT_TIMEOUT"`
	KeepAlive      time.Duration `envconfig:"KEDA_HTTP_KEEP_ALIVE"`
	ResponseHeader time.Duration `envconfig:"KEDA_RESPONSE_HEADER_TIMEOUT"`
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
