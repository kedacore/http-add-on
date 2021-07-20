package config

import (
	"fmt"

	"github.com/kedacore/http-add-on/operator/api/v1alpha1"
)

// DeploymentName is a convenience function for
// a.HTTPScaledObject.Spec.ScaleTargetRef.Deployment
func DeploymentName(httpso v1alpha1.HTTPScaledObject) string {
	return httpso.Spec.ScaleTargetRef.Deployment
}

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

func InterceptorScaledObjectName(httpso *v1alpha1.HTTPScaledObject) string {
	return fmt.Sprintf("%s-interceptor", httpso.Spec.ScaleTargetRef.Deployment)
}
