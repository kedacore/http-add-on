package config

import (
	"fmt"

	"github.com/kedacore/http-add-on/pkg/env"
)

// App holds configuration for the application. It gets this information from the
// HTTPScaledObject, so it's not static
type App struct {
	Name        string
	Port        int32
	Image       string
	Namespace   string
	IngressHost string
}

// Interceptor holds static configuration info for the interceptor
type Interceptor struct {
	Image     string
	ProxyPort int32
	AdminPort int32
}

// NewInterceptorFromEnv gets interceptor configuration values from environment variables and/or
// sensible defaults if values were missing.
// and returns the interceptor struct to match. Returns an error if required values were missing.
func NewInterceptorFromEnv() (*Interceptor, error) {
	image, err := env.Get("KEDAHTTP_OPERATOR_INTERCEPTOR_IMAGE")
	if err != nil {
		return nil, fmt.Errorf("missing KEDAHTTP_OPERATOR_INTERCEPTOR_IMAGE")
	}
	adminPort := env.GetInt32Or("KEDAHTTP_OPERATOR_INTERCEPTOR_ADMIN_PORT", 8090)
	proxyPort := env.GetInt32Or("KEDAHTTP_OPERATOR_INTERCEPTOR_PROXY_PORT", 8091)
	return &Interceptor{
		Image:     image,
		AdminPort: adminPort,
		ProxyPort: proxyPort,
	}, nil
}

// ExternalScaler holds static configuration info for the external scaler
type ExternalScaler struct {
	Image string
	Port  int32
}

// NewExternalScalerFromEnv gets external scaler configuration values from environment variables and/or
// sensible defaults if values were missing.
// and returns the interceptor struct to match. Returns an error if required values were missing.
func NewExternalScalerFromEnv() (*ExternalScaler, error) {
	image, err := env.Get("KEDAHTTP_OPERATOR_EXTERNAL_SCALER_IMAGE")
	port := env.GetInt32Or("KEDAHTTP_OPERATOR_EXTERNAL_SCALER_PORT", 8091)
	if err != nil {
		return nil, fmt.Errorf("Missing KEDAHTTP_EXTERNAL_SCALER_IMAGE")
	}
	return &ExternalScaler{
		Image: image,
		Port:  port,
	}, nil
}
