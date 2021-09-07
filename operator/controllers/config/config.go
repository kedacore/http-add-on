package config

import (
	"fmt"
	"strconv"

	"github.com/kelseyhightower/envconfig"
)

// Interceptor holds static configuration info for the interceptor
type Interceptor struct {
	ServiceName string `envconfig:"INTERCEPTOR_SERVICE_NAME" required:"true"`
	ProxyPort   int32  `envconfig:"INTERCEPTOR_PROXY_PORT" required:"true"`
	AdminPort   int32  `envconfig:"INTERCEPTOR_ADMIN_PORT" required:"true"`
}

// ExternalScaler holds static configuration info for the external scaler
type ExternalScaler struct {
	ServiceName string `envconfig:"EXTERNAL_SCALER_SERVICE_NAME" required:"true"`
	Port        int32  `envconfig:"EXTERNAL_SCALER_PORT" required:"true"`
}

func (e ExternalScaler) HostName(namespace string) string {
	return fmt.Sprintf(
		"%s.%s.svc.cluster.local:%d",
		e.ServiceName,
		namespace,
		e.Port,
	)
}

// AdminPortString returns i.AdminPort in string format, rather than
// as an int32.
func (i Interceptor) AdminPortString() string {
	return strconv.Itoa(int(i.AdminPort))
}

// NewInterceptorFromEnv gets interceptor configuration values from environment variables and/or
// sensible defaults if values were missing.
// and returns the interceptor struct to match. Returns an error if required values were missing.
func NewInterceptorFromEnv() (*Interceptor, error) {
	ret := new(Interceptor)
	if err := envconfig.Process("KEDAHTTP_OPERATOR_INTERCEPTOR", ret); err != nil {
		return nil, err
	}
	return ret, nil

}

// NewExternalScalerFromEnv gets external scaler configuration values from environment variables and/or
// sensible defaults if values were missing.
// and returns the interceptor struct to match. Returns an error if required values were missing.
func NewExternalScalerFromEnv() (*ExternalScaler, error) {
	ret := new(ExternalScaler)
	if err := envconfig.Process("KEDAHTTP_OPERATOR_EXTERNAL_SCALER", ret); err != nil {
		return nil, err
	}
	return ret, nil
}
