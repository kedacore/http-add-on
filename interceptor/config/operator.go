package config

import (
	"fmt"
	"net/url"
	"time"

	"github.com/kelseyhightower/envconfig"
)

// Operator is the configuration for where and how the interceptor
// makes RPC calls to the operator
type Operator struct {
	OperatorServiceName          string `envconfig:"KEDA_HTTP_OPERATOR_SERVICE_NAME" required:"true"`
	OperatorServicePort          string `envconfig:"KEDA_HTTP_OPERATOR_SERVICE_PORT" required:"true"`
	RoutingTableUpdateDurationMS int    `envconfig:"KEDA_HTTP_OPERATOR_ROUTING_TABLE_UPDATE_DURATION_MS" default:"500"`
}

func (o *Operator) RoutingTableUpdateDuration() time.Duration {
	return time.Duration(o.RoutingTableUpdateDurationMS) * time.Millisecond
}

// ServiceURL formats the app service name and port into a URL
func (o *Operator) RoutingFetchURL() (*url.URL, error) {
	urlStr := fmt.Sprintf(
		"http://%s:%s",
		o.OperatorServiceName,
		o.OperatorServicePort,
	)
	u, err := url.Parse(urlStr)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func MustParseOperator() *Operator {
	ret := new(Operator)
	envconfig.MustProcess("", ret)
	return ret
}
