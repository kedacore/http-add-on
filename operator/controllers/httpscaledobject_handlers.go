package controllers

import (
	"fmt"
	"strconv"

	"github.com/go-logr/logr"
	"github.com/kedacore/http-add-on/operator/api/v1alpha1"
	"github.com/kedacore/http-add-on/pkg/k8s"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
)


func createScaledObject (
	appInfo userApplicationInfo,
	K8sDynamicCl dynamic.Interface,
	logger logr.Logger,
	httpso *v1alpha1.HTTPScaledObject,
) error {
	coreScaledObject := k8s.NewScaledObject(
		appInfo.namespace,
		appInfo.name,
		appInfo.name,
		fmt.Sprintf("%s.%s.svc.cluster.local:%d", appInfo.externalScalerName, appInfo.namespace, defaultExposedPort),
	)
	// TODO: use r.Client here, not the dynamic one
	scaledObjectCl := k8s.NewScaledObjectClient(K8sDynamicCl)
	if _, err := scaledObjectCl.
		Namespace(appInfo.namespace).
		Create(coreScaledObject, metav1.CreateOptions{}); err != nil {
		logger.Error(err, "Creating ScaledObject")
		httpso.Status.ScaledObjectStatus = v1alpha1.Error
		return err
	}
	httpso.Status.ScaledObjectStatus = v1alpha1.Created
	return nil
}

func createUserApp (
	appInfo userApplicationInfo,
	clients kubernetesClients,
	logger logr.Logger,
	httpso *v1alpha1.HTTPScaledObject,
) error {
	deployment := k8s.NewDeployment(appInfo.namespace, appInfo.name, appInfo.image, appInfo.port, []corev1.EnvVar{})
	// TODO: watch the deployment until it reaches ready state
	// Option: start the creation here and add another method to check if the resources are created
	if _, err := clients.appsCl.Create(deployment); err != nil {
		logger.Error(err, "Creating deployment")
		httpso.Status.DeploymentStatus = v1alpha1.Error
		return err
	}
	httpso.Status.DeploymentStatus = v1alpha1.Created

	service := k8s.NewService(appInfo.namespace, appInfo.name, appInfo.port)
	if _, err := clients.coreCl.Create(service); err != nil {
		logger.Error(err, "Creating service")
		httpso.Status.ServiceStatus = v1alpha1.Error
		return err
	}
	httpso.Status.ServiceStatus = v1alpha1.Created
	return nil
}

func createInterceptor (
	appInfo userApplicationInfo,
	clients kubernetesClients,
	logger logr.Logger,
	httpso *v1alpha1.HTTPScaledObject,
) error {
	interceptorEnvs := []corev1.EnvVar{
		{
			Name: "KEDA_HTTP_SERVICE_NAME",
			Value: appInfo.name,
		},
		{
			Name: "KEDA_HTTP_SERVICE_PORT",
			Value: strconv.FormatInt(int64(appInfo.port), 10),
		},
	}

	// NOTE: Interceptor port is fixed here because it's a fixed on the interceptor main (@see ../interceptor/main.go:49)
	interceptorDeployment := k8s.NewDeployment(appInfo.namespace, appInfo.interceptorName, imageRegistry + interceptorImageName, int32(defaultExposedPort), interceptorEnvs)
	if _, err := clients.appsCl.Create(interceptorDeployment); err != nil {
		logger.Error(err, "Creating interceptor deployment")
		httpso.Status.InterceptorStatus = v1alpha1.Error
		return err
	}

	// NOTE: Interceptor port is fixed here because it's a fixed on the interceptor main (@see ../interceptor/main.go:49)
	interceptorService := k8s.NewService(appInfo.namespace, appInfo.interceptorName, int32(defaultExposedPort))
	if _, err := clients.coreCl.Create(interceptorService); err != nil {
		logger.Error(err, "Creating interceptor service")
		httpso.Status.InterceptorStatus = v1alpha1.Error
		return err
	}
	httpso.Status.InterceptorStatus = v1alpha1.Created
	return nil
}

func createExternalScaler (
	appInfo userApplicationInfo,
	clients kubernetesClients,
	logger logr.Logger,
	httpso *v1alpha1.HTTPScaledObject,
) error {
	// NOTE: Scaler port is fixed here because it's a fixed on the scaler main (@see ../scaler/main.go:17)
	scalerDeployment := k8s.NewDeployment(appInfo.namespace, appInfo.externalScalerName, imageRegistry + externalScalerImageName, int32(defaultExposedPort), []corev1.EnvVar{})
	if _, err := clients.appsCl.Create(scalerDeployment); err != nil {
		logger.Error(err, "Creating scaler deployment")
		httpso.Status.ExternalScalerStatus = v1alpha1.Error
		return err
	}

	// NOTE: Scaler port is fixed here because it's a fixed on the scaler main (@see ../scaler/main.go:17)
	scalerService := k8s.NewService(appInfo.namespace, appInfo.externalScalerName, int32(defaultExposedPort))
	if _, err := clients.coreCl.Create(scalerService); err != nil {
		logger.Error(err, "Creating scaler service")
		httpso.Status.ExternalScalerStatus = v1alpha1.Error
		return err
	}
	httpso.Status.ExternalScalerStatus = v1alpha1.Created
	return nil
}
