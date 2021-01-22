package controllers

import (
	"fmt"
	"strconv"

	"github.com/go-logr/logr"
	"github.com/kedacore/http-add-on/operator/api/v1alpha1"
	"github.com/kedacore/http-add-on/operator/controllers/config"
	"github.com/kedacore/http-add-on/pkg/k8s"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

func createScaledObject(
	appInfo config.AppInfo,
	K8sDynamicCl dynamic.Interface,
	logger logr.Logger,
	httpso *v1alpha1.HTTPScaledObject,
) error {

	coreScaledObject := k8s.NewScaledObject(
		appInfo.ScaledObjectName(),
		appInfo.ScaledObjectName(),
		fmt.Sprintf(
			"%s.%s.svc.cluster.local:%d",
			appInfo.ExternalScalerServiceName(),
			appInfo.Namespace,
			appInfo.ExternalScalerPort,
		),
	)
	logger.Info("Creating ScaledObject", "ScaledObject", *coreScaledObject)
	// TODO: use r.Client here, not the dynamic one
	scaledObjectCl := k8s.NewScaledObjectClient(K8sDynamicCl)
	if _, err := scaledObjectCl.
		Namespace(appInfo.Namespace).
		Create(coreScaledObject, metav1.CreateOptions{}); err != nil {
		logger.Error(err, "Creating ScaledObject")
		httpso.Status.ScaledObjectStatus = v1alpha1.Error
		return err
	}
	httpso.Status.ScaledObjectStatus = v1alpha1.Created
	return nil
}

func createUserApp(
	appInfo config.AppInfo,
	cl *kubernetes.Clientset,
	logger logr.Logger,
	httpso *v1alpha1.HTTPScaledObject,
) error {
	deployment := k8s.NewDeployment(appInfo.Name, appInfo.Image, appInfo.Port, []corev1.EnvVar{})
	logger.Info("Creating app deployment", "deployment", *deployment)
	// TODO: watch the deployment until it reaches ready state
	// Option: start the creation here and add another method to check if the resources are created
	deploymentsCl := cl.AppsV1().Deployments(appInfo.Namespace)
	if _, err := deploymentsCl.Create(deployment); err != nil {
		logger.Error(err, "Creating deployment")
		httpso.Status.DeploymentStatus = v1alpha1.Error
		return err
	}
	httpso.Status.DeploymentStatus = v1alpha1.Created

	service := k8s.NewService(appInfo.Name, appInfo.Port)
	servicesCl := cl.CoreV1().Services(appInfo.Namespace)
	if _, err := servicesCl.Create(service); err != nil {
		logger.Error(err, "Creating service")
		httpso.Status.ServiceStatus = v1alpha1.Error
		return err
	}
	httpso.Status.ServiceStatus = v1alpha1.Created
	return nil
}

func createInterceptor(
	appInfo config.AppInfo,
	cl *kubernetes.Clientset,
	logger logr.Logger,
	httpso *v1alpha1.HTTPScaledObject,
) error {
	interceptorEnvs := []corev1.EnvVar{
		{
			Name:  "KEDA_HTTP_SERVICE_NAME",
			Value: appInfo.Name,
		},
		{
			Name:  "KEDA_HTTP_SERVICE_PORT",
			Value: strconv.FormatInt(int64(appInfo.Port), 10),
		},
	}

	// NOTE: Interceptor port is fixed here because it's a fixed on the interceptor main (@see ../interceptor/main.go:49)
	interceptorDeployment := k8s.NewDeployment(
		appInfo.InterceptorDeploymentName(),
		appInfo.InterceptorImage,
		appInfo.InterceptorPort,
		interceptorEnvs,
	)
	logger.Info("Creating interceptor Deployment", "Deployment", *interceptorDeployment)
	deploymentsCl := cl.AppsV1().Deployments(appInfo.Namespace)
	if _, err := deploymentsCl.Create(interceptorDeployment); err != nil {
		logger.Error(err, "Creating interceptor deployment")
		httpso.Status.InterceptorStatus = v1alpha1.Error
		return err
	}

	// NOTE: Interceptor port is fixed here because it's a fixed on the interceptor main (@see ../interceptor/main.go:49)
	interceptorService := k8s.NewService(appInfo.InterceptorServiceName(), appInfo.InterceptorPort)
	servicesCl := cl.CoreV1().Services(appInfo.Namespace)
	if _, err := servicesCl.Create(interceptorService); err != nil {
		logger.Error(err, "Creating interceptor service")
		httpso.Status.InterceptorStatus = v1alpha1.Error
		return err
	}
	httpso.Status.InterceptorStatus = v1alpha1.Created
	return nil
}

func createExternalScaler(
	appInfo config.AppInfo,
	cl *kubernetes.Clientset,
	logger logr.Logger,
	httpso *v1alpha1.HTTPScaledObject,
) error {
	// NOTE: Scaler port is fixed here because it's a fixed on the scaler main (@see ../scaler/main.go:17)
	scalerDeployment := k8s.NewDeployment(
		appInfo.ExternalScalerDeploymentName(),
		appInfo.ExternalScalerImage,
		appInfo.ExternalScalerPort,
		[]corev1.EnvVar{
			{
				Name:  "KEDA_HTTP_SCALER_PORT",
				Value: fmt.Sprintf("%d", appInfo.ExternalScalerPort),
			},
			{
				Name:  "KEDA_HTTP_SCALER_TARGET_ADMIN_NAMESPACE",
				Value: appInfo.Namespace,
			},
			{
				Name:  "KEDA_HTTP_SCALER_TARGET_ADMIN_SERVICE",
				Value: appInfo.ExternalScalerServiceName(),
			},
			{
				Name:  "KEDA_HTTP_SCALER_TARGET_ADMIN_PORT",
				Value: fmt.Sprintf("%d", appInfo.InterceptorPort),
			},
		},
	)
	logger.Info("Creating external scaler Deployment", "Deployment", *scalerDeployment)
	deploymentsCl := cl.AppsV1().Deployments(appInfo.Namespace)
	if _, err := deploymentsCl.Create(scalerDeployment); err != nil {
		logger.Error(err, "Creating scaler deployment")
		httpso.Status.ExternalScalerStatus = v1alpha1.Error
		return err
	}

	// NOTE: Scaler port is fixed here because it's a fixed on the scaler main (@see ../scaler/main.go:17)
	scalerService := k8s.NewService(appInfo.ExternalScalerServiceName(), appInfo.ExternalScalerPort)
	logger.Info("Creating external scaler Service", "Service", *scalerService)
	servicesCl := cl.CoreV1().Services(appInfo.Namespace)
	if _, err := servicesCl.Create(scalerService); err != nil {
		logger.Error(err, "Creating scaler service")
		httpso.Status.ExternalScalerStatus = v1alpha1.Error
		return err
	}
	httpso.Status.ExternalScalerStatus = v1alpha1.Created
	return nil
}
