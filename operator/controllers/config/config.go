package config

import (
	"fmt"

	"github.com/kelseyhightower/envconfig"
)

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
