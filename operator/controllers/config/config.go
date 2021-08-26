package config

import (
	"fmt"

	"github.com/kelseyhightower/envconfig"
	corev1 "k8s.io/api/core/v1"
)

// Interceptor holds static configuration info for the interceptor
type Interceptor struct {
	Image      string            `envconfig:"IMAGE" required:"true"`
	ProxyPort  int32             `envconfig:"PROXY_PORT" default:"8091"`
	AdminPort  int32             `envconfig:"ADMIN_PORT" default:"8090"`
	PullPolicy corev1.PullPolicy `envconfig:"PULL_POLICY" required:"true"`
}

// ExternalScaler holds static configuration info for the external scaler
type ExternalScaler struct {
	Image      string            `envconfig:"IMAGE" required:"true"`
	Port       int32             `envconfig:"PORT" default:"8091"`
	PullPolicy corev1.PullPolicy `envconfig:"PULL_POLICY" default:"Always"`
}

func ensureValidPolicy(policy string) error {
	converted := corev1.PullPolicy(policy)
	switch converted {
	case corev1.PullAlways, corev1.PullIfNotPresent, corev1.PullNever:
		return nil
	}
	return fmt.Errorf(
		"policy %q is not a valid Pull Policy. Accepted values are: %s, %s, %s",
		policy,
		corev1.PullAlways,
		corev1.PullIfNotPresent,
		corev1.PullNever,
	)
}

// NewInterceptorFromEnv gets interceptor configuration values from environment variables and/or
// sensible defaults if values were missing.
// and returns the interceptor struct to match. Returns an error if required values were missing.
func NewInterceptorFromEnv() (*Interceptor, error) {
	ret := new(Interceptor)
	if err := envconfig.Process("KEDAHTTP_OPERATOR_INTERCEPTOR", ret); err != nil {
		return nil, err
	}
	if err := ensureValidPolicy(string(ret.PullPolicy)); err != nil {
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
	if err := ensureValidPolicy(string(ret.PullPolicy)); err != nil {
		return nil, err
	}
	return ret, nil
}
