package controllers

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/kedacore/http-add-on/operator/api/v1alpha1"
	"github.com/kedacore/http-add-on/operator/controllers/config"
	"github.com/kedacore/http-add-on/pkg/k8s"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func createExternalScaler(
	ctx context.Context,
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
				Value: appInfo.InterceptorAdminServiceName(),
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
	if _, err := deploymentsCl.Create(ctx, scalerDeployment, metav1.CreateOptions{}); err != nil {
		if errors.IsAlreadyExists(err) {
			logger.Info("External scaler deployment already exists, moving on")
		} else {
			logger.Error(err, "Creating scaler deployment")
			condition := v1alpha1.CreateCondition(v1alpha1.Error, metav1.ConditionFalse, v1alpha1.ErrorCreatingExternalScaler).SetMessage(err.Error())
			httpso.AddCondition(*condition)
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
	if _, err := servicesCl.Create(ctx, scalerService, metav1.CreateOptions{}); err != nil {
		if errors.IsAlreadyExists(err) {
			logger.Info("External scaler service already exists, moving on")
		} else {
			logger.Error(err, "Creating scaler service")
			condition := v1alpha1.CreateCondition(v1alpha1.Error, metav1.ConditionFalse, v1alpha1.ErrorCreatingExternalScalerService).SetMessage(err.Error())
			httpso.AddCondition(*condition)
			return err
		}
	}
	condition := v1alpha1.CreateCondition(v1alpha1.Created, metav1.ConditionTrue, v1alpha1.CreatedExternalScaler).SetMessage("External scaler object is created")
	httpso.AddCondition(*condition)
	return nil
}
