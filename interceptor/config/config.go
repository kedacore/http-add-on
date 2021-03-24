package config

import (
	"time"

	"github.com/kelseyhightower/envconfig"
)

type Timeouts struct {
	Connect        time.Duration `envconfig:"KEDA_HTTP_CONNECT_TIMEOUT"`
	KeepAlive      time.Duration `envconfig:"KEDA_HTTP_KEEP_ALIVE"`
	ResponseHeader time.Duration `envconfig:"KEDA_RESPONSE_HEADER_TIMEOUT"`
}

func ParseTimeouts() (*Timeouts, error) {
	ret := new(Timeouts)
	if err := envconfig.Process("", ret); err != nil {
		return nil, err
	}
	return ret, nil
}
