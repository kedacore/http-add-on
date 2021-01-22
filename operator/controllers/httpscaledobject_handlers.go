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
	"k8s.io/client-go/kubernetes"
)

func createScaledObject(
	appInfo userApplicationInfo,
	K8sDynamicCl dynamic.Interface,
	logger logr.Logger,
	httpso *v1alpha1.HTTPScaledObject,
) error {

	coreScaledObject := k8s.NewScaledObject(
		appInfo.name,
		appInfo.name,
		fmt.Sprintf(
			"%s.%s.svc.cluster.local:%d",
			appInfo.externalScalerName,
			appInfo.namespace,
			appInfo.externalScalerPort,
		),
	)
	logger.Info("Creating ScaledObject", "ScaledObject", *coreScaledObject)
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

func createUserApp(
	appInfo userApplicationInfo,
	cl *kubernetes.Clientset,
	logger logr.Logger,
	httpso *v1alpha1.HTTPScaledObject,
) error {
	deployment := k8s.NewDeployment(appInfo.name, appInfo.image, appInfo.port, []corev1.EnvVar{})
	logger.Info("Creating app deployment", "deployment", *deployment)
	// TODO: watch the deployment until it reaches ready state
	// Option: start the creation here and add another method to check if the resources are created
	deploymentsCl := cl.AppsV1().Deployments(appInfo.namespace)
	if _, err := deploymentsCl.Create(deployment); err != nil {
		logger.Error(err, "Creating deployment")
		httpso.Status.DeploymentStatus = v1alpha1.Error
		return err
	}
	httpso.Status.DeploymentStatus = v1alpha1.Created

	service := k8s.NewService(appInfo.name, appInfo.port)
	servicesCl := cl.CoreV1().Services(appInfo.namespace)
	if _, err := servicesCl.Create(service); err != nil {
		logger.Error(err, "Creating service")
		httpso.Status.ServiceStatus = v1alpha1.Error
		return err
	}
	httpso.Status.ServiceStatus = v1alpha1.Created
	return nil
}

func createInterceptor(
	appInfo userApplicationInfo,
	cl *kubernetes.Clientset,
	logger logr.Logger,
	httpso *v1alpha1.HTTPScaledObject,
) error {
	interceptorEnvs := []corev1.EnvVar{
		{
			Name:  "KEDA_HTTP_SERVICE_NAME",
			Value: appInfo.name,
		},
		{
			Name:  "KEDA_HTTP_SERVICE_PORT",
			Value: strconv.FormatInt(int64(appInfo.port), 10),
		},
	}

	// NOTE: Interceptor port is fixed here because it's a fixed on the interceptor main (@see ../interceptor/main.go:49)
	interceptorDeployment := k8s.NewDeployment(
		appInfo.interceptorName,
		appInfo.interceptorImage,
		appInfo.interceptorPort,
		interceptorEnvs,
	)
	logger.Info("Creating interceptor Deployment", "Deployment", *interceptorDeployment)
	deploymentsCl := cl.AppsV1().Deployments(appInfo.namespace)
	if _, err := deploymentsCl.Create(interceptorDeployment); err != nil {
		logger.Error(err, "Creating interceptor deployment")
		httpso.Status.InterceptorStatus = v1alpha1.Error
		return err
	}

	// NOTE: Interceptor port is fixed here because it's a fixed on the interceptor main (@see ../interceptor/main.go:49)
	interceptorService := k8s.NewService(appInfo.interceptorName, appInfo.interceptorPort)
	servicesCl := cl.CoreV1().Services(appInfo.namespace)
	if _, err := servicesCl.Create(interceptorService); err != nil {
		logger.Error(err, "Creating interceptor service")
		httpso.Status.InterceptorStatus = v1alpha1.Error
		return err
	}
	httpso.Status.InterceptorStatus = v1alpha1.Created
	return nil
}

func createExternalScaler(
	appInfo userApplicationInfo,
	cl *kubernetes.Clientset,
	logger logr.Logger,
	httpso *v1alpha1.HTTPScaledObject,
) error {
	// NOTE: Scaler port is fixed here because it's a fixed on the scaler main (@see ../scaler/main.go:17)
	scalerDeployment := k8s.NewDeployment(
		appInfo.externalScalerName,
		appInfo.externalScalerImage,
		appInfo.externalScalerPort,
		[]corev1.EnvVar{},
	)
	logger.Info("Creating external scaler Deployment", "Deployment", *scalerDeployment)
	deploymentsCl := cl.AppsV1().Deployments(appInfo.namespace)
	if _, err := deploymentsCl.Create(scalerDeployment); err != nil {
		logger.Error(err, "Creating scaler deployment")
		httpso.Status.ExternalScalerStatus = v1alpha1.Error
		return err
	}

	// NOTE: Scaler port is fixed here because it's a fixed on the scaler main (@see ../scaler/main.go:17)
	scalerService := k8s.NewService(appInfo.externalScalerName, appInfo.externalScalerPort)
	logger.Info("Creating external scaler Service", "Service", *scalerService)
	servicesCl := cl.CoreV1().Services(appInfo.namespace)
	if _, err := servicesCl.Create(scalerService); err != nil {
		logger.Error(err, "Creating scaler service")
		httpso.Status.ExternalScalerStatus = v1alpha1.Error
		return err
	}
	httpso.Status.ExternalScalerStatus = v1alpha1.Created
	return nil
}
