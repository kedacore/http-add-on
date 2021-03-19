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

// ExternalScalerServiceName is a convenience method to get the name of the external scaler
// service in Kubernetes
func (a AppInfo) ExternalScalerServiceName() string {
	return fmt.Sprintf("%s-external-scaler", a.Name)
}

// ExternalScalerDeploymentName is a convenience method to get the name of the external scaler
// deployment in Kubernetes
func (a AppInfo) ExternalScalerDeploymentName() string {
	return fmt.Sprintf("%s-external-scaler", a.Name)
}

// InterceptorAdminServiceName is a convenience method to get the name of the interceptor
// service for the admin endpoints in Kubernetes
func (a AppInfo) InterceptorAdminServiceName() string {
	return fmt.Sprintf("%s-interceptor-admin", a.Name)
}

// InterceptorProxyServiceName is a convenience method to get the name of the interceptor
// service for the proxy in Kubernetes
func (a AppInfo) InterceptorProxyServiceName() string {
	return fmt.Sprintf("%s-interceptor-proxy", a.Name)
}

// InterceptorDeploymentName is a convenience method to get the name of the interceptor
// deployment in Kubernetes
func (a AppInfo) InterceptorDeploymentName() string {
	return fmt.Sprintf("%s-interceptor", a.Name)
}

func AppScaledObjectName(httpso *v1alpha1.HTTPScaledObject) string {
	return fmt.Sprintf("%s-app", httpso.Spec.ScaleTargetRef.Deployment)
}

func InterceptorScaledObjectName(httpso *v1alpha1.HTTPScaledObject) string {
	return fmt.Sprintf("%s-interceptor", httpso.Spec.ScaleTargetRef.Deployment)
}
