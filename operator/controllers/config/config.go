package config

import (
	"fmt"
	"strconv"

	"github.com/kedacore/http-add-on/pkg/env"
)

// Interceptor holds static configuration info for the interceptor
type Interceptor struct {
	ServiceName string
	ProxyPort   int32
	AdminPort   int32
}

// ExternalScaler holds static configuration info for the external scaler
type ExternalScaler struct {
	ServiceName string
	Port        int32
}

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
