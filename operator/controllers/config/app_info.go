package config

import "fmt"

type AppInfo struct {
	Name                 string
	Port                 int32
	Image                string
	Namespace            string
	InterceptorConfig    Interceptor
	ExternalScalerConfig ExternalScaler
}

func (a AppInfo) ExternalScalerServiceName() string {
	return fmt.Sprintf("%s-external-scaler-service", a.Name)
}

func (a AppInfo) ExternalScalerDeploymentName() string {
	return fmt.Sprintf("%s-external-scaler-deployment", a.Name)
}

func (a AppInfo) InterceptorServiceName() string {
	return fmt.Sprintf("%s-interceptor-service", a.Name)
}

func (a AppInfo) InterceptorDeploymentName() string {
	return fmt.Sprintf("%s-interceptor-deployment", a.Name)
}

func (a AppInfo) ScaledObjectName() string {
	return fmt.Sprintf("%s-scaled-object", a.Name)
}
