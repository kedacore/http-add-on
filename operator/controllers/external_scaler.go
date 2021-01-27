package controllers

import (
	"fmt"

	"github.com/go-logr/logr"
	"github.com/kedacore/http-add-on/operator/api/v1alpha1"
	"github.com/kedacore/http-add-on/operator/controllers/config"
	"github.com/kedacore/http-add-on/pkg/k8s"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"
)

func createExternalScaler(
	appInfo config.AppInfo,
	cl *kubernetes.Clientset,
	logger logr.Logger,
	httpso *v1alpha1.HTTPScaledObject,
) error {
	// NOTE: Scaler port is fixed here because it's a fixed on the scaler main (@see ../scaler/main.go:17)
	scalerDeployment := k8s.NewDeployment(
		appInfo.ExternalScalerDeploymentName(),
		appInfo.ExternalScalerConfig.Image,
		[]int32{
			appInfo.ExternalScalerConfig.Port,
		},
		[]corev1.EnvVar{
			{
				Name:  "KEDA_HTTP_SCALER_PORT",
				Value: fmt.Sprintf("%d", appInfo.ExternalScalerConfig.Port),
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
				Value: fmt.Sprintf("%d", appInfo.InterceptorConfig.AdminPort),
			},
		},
		k8s.Labels(appInfo.ExternalScalerDeploymentName()),
	)
	logger.Info("Creating external scaler Deployment", "Deployment", *scalerDeployment)
	deploymentsCl := cl.AppsV1().Deployments(appInfo.Namespace)
	if _, err := deploymentsCl.Create(scalerDeployment); err != nil {
		if errors.IsAlreadyExists(err) {
			logger.Info("External scaler deployment already exists, moving on")
		} else {
			logger.Error(err, "Creating scaler deployment")
			httpso.Status.ExternalScalerStatus = v1alpha1.Error
			return err
		}
	}

	// NOTE: Scaler port is fixed here because it's a fixed on the scaler main (@see ../scaler/main.go:17)
	servicePorts := []corev1.ServicePort{
		k8s.NewTCPServicePort(
			"externalscaler",
			appInfo.ExternalScalerConfig.Port,
			appInfo.ExternalScalerConfig.Port,
		),
	}
	scalerService := k8s.NewService(
		appInfo.ExternalScalerServiceName(),
		servicePorts,
		corev1.ServiceTypeClusterIP,
		k8s.Labels(appInfo.ExternalScalerDeploymentName()),
	)
	logger.Info("Creating external scaler Service", "Service", *scalerService)
	servicesCl := cl.CoreV1().Services(appInfo.Namespace)
	if _, err := servicesCl.Create(scalerService); err != nil {
		if errors.IsAlreadyExists(err) {
			logger.Info("External scaler service already exists, moving on")
		} else {
			logger.Error(err, "Creating scaler service")
			httpso.Status.ExternalScalerStatus = v1alpha1.Error
			return err
		}
	}
	httpso.Status.ExternalScalerStatus = v1alpha1.Created
	return nil
}
