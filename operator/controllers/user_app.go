package controllers

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/kedacore/http-add-on/operator/api/v1alpha1"
	"github.com/kedacore/http-add-on/operator/controllers/config"
	"github.com/kedacore/http-add-on/pkg/k8s"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func createUserApp(
	ctx context.Context,
	appInfo config.AppInfo,
	cl *kubernetes.Clientset,
	logger logr.Logger,
	httpso *v1alpha1.HTTPScaledObject,
) error {
	deployment := k8s.NewDeployment(
		appInfo.Name,
		appInfo.Image,
		[]int32{appInfo.Port},
		[]corev1.EnvVar{},
		k8s.Labels(appInfo.Name),
	)
	logger.Info("Creating app deployment", "deployment", *deployment)
	// TODO: watch the deployment until it reaches ready state
	// Option: start the creation here and add another method to check if the resources are created
	deploymentsCl := cl.AppsV1().Deployments(appInfo.Namespace)
	if _, err := deploymentsCl.Create(ctx, deployment, v1.CreateOptions{}); err != nil {
		if errors.IsAlreadyExists(err) {
			logger.Info("User app deployment already exists, moving on")
		} else {
			logger.Error(err, "Creating deployment")
			condition := v1alpha1.CreateCondition(v1alpha1.Error, v1.ConditionFalse, v1alpha1.ErrorCreatingAppDeployment).SetMessage(err.Error())
			httpso.AddCondition(*condition)
			return err
		}
	}
	httpso.AddCondition(*v1alpha1.CreateCondition(v1alpha1.Created, v1.ConditionTrue, v1alpha1.AppDeploymentCreated).SetMessage("App deployment created"))

	servicePorts := []corev1.ServicePort{
		k8s.NewTCPServicePort("http", 8080, appInfo.Port),
	}
	service := k8s.NewService(
		appInfo.Name,
		servicePorts,
		corev1.ServiceTypeClusterIP,
		k8s.Labels(appInfo.Name),
	)
	servicesCl := cl.CoreV1().Services(appInfo.Namespace)
	if _, err := servicesCl.Create(ctx, service, v1.CreateOptions{}); err != nil {
		if errors.IsAlreadyExists(err) {
			logger.Info("User app service already exists, moving on")
		} else {
			logger.Error(err, "Creating service")
			condition := v1alpha1.CreateCondition(v1alpha1.Error, v1.ConditionFalse, v1alpha1.ErrorCreatingAppService).SetMessage(err.Error())
			httpso.AddCondition(*condition)
			return err
		}
	}
	httpso.AddCondition(*v1alpha1.CreateCondition(v1alpha1.Created, v1.ConditionTrue, v1alpha1.AppServiceCreated).SetMessage("App service created"))
	return nil
}
