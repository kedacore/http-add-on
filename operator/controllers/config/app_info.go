package config

import "fmt"

type AppInfo struct {
	Name                 string
	Port                 int32
	Image                string
	MinReplicas          int32
	MaxReplicas          int32
	Namespace            string
	InterceptorConfig    Interceptor
	ExternalScalerConfig ExternalScaler
}

func (a AppInfo) ExternalScalerServiceName() string {
	return fmt.Sprintf("%s-external-scaler", a.Name)
}

func (a AppInfo) ExternalScalerDeploymentName() string {
	return fmt.Sprintf("%s-external-scaler", a.Name)
}

func (a AppInfo) InterceptorAdminServiceName() string {
	return fmt.Sprintf("%s-interceptor-admin", a.Name)
}

func (a AppInfo) InterceptorProxyServiceName() string {
	return fmt.Sprintf("%s-interceptor-proxy", a.Name)
}

func (a AppInfo) InterceptorDeploymentName() string {
	return fmt.Sprintf("%s-interceptor", a.Name)
}

func (a AppInfo) ScaledObjectName() string {
	return fmt.Sprintf("%s-scaled-object", a.Name)
}
