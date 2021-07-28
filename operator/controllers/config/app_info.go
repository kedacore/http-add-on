package config

import (
	"fmt"

	"github.com/kedacore/http-add-on/operator/api/v1alpha1"
)

// AppInfo contains configuration for the Interceptor and External Scaler, and holds
// data about the name and namespace of the scale target.
type AppInfo struct {
	Name                 string
	Namespace            string
	InterceptorConfig    Interceptor
	ExternalScalerConfig ExternalScaler
}

func AppScaledObjectName(httpso *v1alpha1.HTTPScaledObject) string {
	return fmt.Sprintf("%s-app", httpso.Spec.ScaleTargetRef.Deployment)
}
