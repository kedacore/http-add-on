package config

import (
	"fmt"
	"net/url"

	"github.com/kelseyhightower/envconfig"
)

// Origin is the configuration for where and how the proxy forwards
// requests to a backing Kubernetes service
type Origin struct {
	// AppServiceName is the name of the service that fronts the user's app
	AppServiceName string `envconfig:"KEDA_HTTP_APP_SERVICE_NAME" required:"true"`
	// AppServiecPort the port that that the proxy should forward to
	AppServicePort string `envconfig:"KEDA_HTTP_APP_SERVICE_PORT" required:"true"`
	// TargetDeploymentName is the name of the backing deployment that the interceptor
	// should forward to
	TargetDeploymentName string `envconfig:"KEDA_HTTP_TARGET_DEPLOYMENT_NAME" required:"true"`
	// Namespace is the namespace that this interceptor is running in
	Namespace string `envconfig:"KEDA_HTTP_NAMESPACE" required:"true"`
}

// ServiceURL formats the app service name and port into a URL
func (o *Origin) ServiceURL() (*url.URL, error) {
	urlStr := fmt.Sprintf("http://%s:%s", o.AppServiceName, o.AppServicePort)
	u, err := url.Parse(urlStr)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func MustParseOrigin() *Origin {
	ret := new(Origin)
	envconfig.MustProcess("", ret)
	return ret
}
