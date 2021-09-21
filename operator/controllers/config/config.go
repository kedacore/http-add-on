package config

import (
	"fmt"
	"strconv"

	"github.com/kedacore/http-add-on/pkg/env"
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

type Base struct {
	TargetPendingRequests int32 `envconfig:"TARGET_PENDING_REQUESTS" default:"100"`
}

func NewBaseFromEnv() (*Base, error) {
	ret := new(Base)
	if err := envconfig.Process(
		"KEDA_HTTP_OPERATOR",
		ret,
	); err != nil {
		return nil, err
	}
	return ret, nil
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
	serviceName, err := env.Get("KEDAHTTP_INTERCEPTOR_SERVICE")
	if err != nil {
		return nil, fmt.Errorf("missing 'KEDAHTTP_INTERCEPTOR_SERVICE'")
	}
	adminPort := env.GetInt32Or("KEDAHTTP_INTERCEPTOR_ADMIN_PORT", 8090)
	proxyPort := env.GetInt32Or("KEDAHTTP_INTERCEPTOR_PROXY_PORT", 8091)

	return &Interceptor{
		ServiceName: serviceName,
		AdminPort:   adminPort,
		ProxyPort:   proxyPort,
	}, nil
}

// NewExternalScalerFromEnv gets external scaler configuration values from environment variables and/or
// sensible defaults if values were missing.
// and returns the interceptor struct to match. Returns an error if required values were missing.
func NewExternalScalerFromEnv() (*ExternalScaler, error) {
	// image, err := env.Get("KEDAHTTP_OPERATOR_EXTERNAL_SCALER_IMAGE")
	serviceName, err := env.Get("KEDAHTTP_OPERATOR_EXTERNAL_SCALER_SERVICE")
	if err != nil {
		return nil, fmt.Errorf("missing KEDAHTTP_EXTERNAL_SCALER_SERVICE")
	}
	port := env.GetInt32Or("KEDAHTTP_OPERATOR_EXTERNAL_SCALER_PORT", 8091)
	return &ExternalScaler{
		ServiceName: serviceName,
		Port:        port,
	}, nil
}
