package config

import (
	"fmt"

	"github.com/kedacore/http-add-on/pkg/env"
	corev1 "k8s.io/api/core/v1"
)

// Interceptor holds static configuration info for the interceptor
type Interceptor struct {
	Image     string
	ProxyPort int32
	AdminPort int32
	PullPolicy corev1.PullPolicy
}

func ensureValidPolicy (policy string) error {
	converted := corev1.PullPolicy(policy)
	switch (converted) {
	case corev1.PullAlways, corev1.PullIfNotPresent, corev1.PullNever:
		return nil
	}
	return fmt.Errorf("Policy %q is not a valid Pull Policy. Accepted values are: %s, %s, %s", policy, corev1.PullAlways, corev1.PullIfNotPresent, corev1.PullNever)
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
	pullPolicy := env.GetOr("INTERCEPTOR_PULL_POLICY", "Always")
	if policyErr := ensureValidPolicy(pullPolicy); policyErr != nil {
		return nil, policyErr
	}

	return &Interceptor{
		Image:     image,
		AdminPort: adminPort,
		ProxyPort: proxyPort,
		PullPolicy: corev1.PullPolicy(pullPolicy),
	}, nil
}

// ExternalScaler holds static configuration info for the external scaler
type ExternalScaler struct {
	Image string
	Port  int32
	PullPolicy corev1.PullPolicy
}

// NewExternalScalerFromEnv gets external scaler configuration values from environment variables and/or
// sensible defaults if values were missing.
// and returns the interceptor struct to match. Returns an error if required values were missing.
func NewExternalScalerFromEnv() (*ExternalScaler, error) {
	image, err := env.Get("KEDAHTTP_OPERATOR_EXTERNAL_SCALER_IMAGE")
	port := env.GetInt32Or("KEDAHTTP_OPERATOR_EXTERNAL_SCALER_PORT", 8091)
	pullPolicy := env.GetOr("SCALER_PULL_POLICY", "Always")
	if err != nil {
		return nil, fmt.Errorf("Missing KEDAHTTP_EXTERNAL_SCALER_IMAGE")
	}
	pullPolicy := env.GetOr("SCALER_PULL_POLICY", "Always")
	if policyErr := ensureValidPolicy(pullPolicy); policyErr != nil {
		return nil, policyErr
	}

	return &ExternalScaler{
		Image: image,
		Port:  port,
		PullPolicy: corev1.PullPolicy(pullPolicy),
	}, nil
}
