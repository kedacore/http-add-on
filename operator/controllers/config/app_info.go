package config

import "fmt"

// AppInfo holds both static and dynamic configuration data about a given application
type AppInfo struct {
	App                  App
	InterceptorConfig    Interceptor
	ExternalScalerConfig ExternalScaler
}

// ExternalScalerServiceName returns the name of the external scaler's Service resource
func (a AppInfo) ExternalScalerServiceName() string {
	return fmt.Sprintf("%s-external-scaler", a.App.Name)
}

// ExternalScalerDeploymentName returns the name of the external scaler's Deployment resource
func (a AppInfo) ExternalScalerDeploymentName() string {
	return fmt.Sprintf("%s-external-scaler", a.App.Name)
}

// InterceptorAdminServiceName returns the name of the interceptor's administrative Service
// resource
func (a AppInfo) InterceptorAdminServiceName() string {
	return fmt.Sprintf("%s-interceptor-admin", a.App.Name)
}

// InterceptorProxyServiceName returns the name of the interceptor's public proxy Service
// resource
func (a AppInfo) InterceptorProxyServiceName() string {
	return fmt.Sprintf("%s-interceptor-proxy", a.App.Name)
}

// InterceptorDeploymentName returns the name of the interceptor's deployment Resource
func (a AppInfo) InterceptorDeploymentName() string {
	return fmt.Sprintf("%s-interceptor", a.App.Name)
}

// InterceptorIngressName returns the name of the interceptor's Ingress resource
func (a AppInfo) InterceptorIngressName() string {
	return fmt.Sprintf("%s-ingress", a.App.Name)
}

// ScaledObjectName returns the name of the interceptor's ScaledObject resource
func (a AppInfo) ScaledObjectName() string {
	return fmt.Sprintf("%s-scaled-object", a.App.Name)
}
