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
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func createInterceptor(
	ctx context.Context,
	appInfo config.AppInfo,
	cl client.Client,
	logger logr.Logger,
	httpso *v1alpha1.HTTPScaledObject,
) error {
	interceptorEnvs := []corev1.EnvVar{
		// timeouts all have reasonable defaults in the interceptor config

		// config regarding the origin
		{
			Name:  "KEDA_HTTP_APP_SERVICE_NAME",
			Value: httpso.Spec.Service.Name,
		},
		{
			Name:  "KEDA_HTTP_APP_SERVICE_PORT",
			Value: fmt.Sprintf("%d", httpso.Spec.Service.Port),
		},
		{
			Name:  "KEDA_HTTP_TARGET_DEPLOYMENT_NAME",
			Value: httpso.Spec.ScaleTargetRef.Deployment,
		},
		{
			Name:  "KEDA_HTTP_NAMESPACE",
			Value: httpso.Namespace,
		},
		// config about how the interceptor should serve
		{
			Name:  "KEDA_HTTP_PROXY_PORT",
			Value: fmt.Sprintf("%d", appInfo.InterceptorConfig.ProxyPort),
		},
		{
			Name:  "KEDA_HTTP_ADMIN_PORT",
			Value: fmt.Sprintf("%d", appInfo.InterceptorConfig.AdminPort),
		},
	}

	deployment := k8s.NewDeployment(
		appInfo.Namespace,
		appInfo.InterceptorDeploymentName(),
		appInfo.InterceptorConfig.Image,
		[]int32{
			appInfo.InterceptorConfig.AdminPort,
			appInfo.InterceptorConfig.ProxyPort,
		},
		interceptorEnvs,
		k8s.Labels(appInfo.InterceptorDeploymentName()),
	)
	logger.Info("Creating interceptor Deployment", "Deployment", *deployment)
	if err := cl.Create(ctx, deployment); err != nil {
		if errors.IsAlreadyExists(err) {
			logger.Info("Interceptor deployment already exists, moving on")
		} else {
			logger.Error(err, "Creating interceptor deployment")
			httpso.AddCondition(*v1alpha1.CreateCondition(v1alpha1.Error, metav1.ConditionFalse, v1alpha1.ErrorCreatingInterceptor).SetMessage(err.Error()))
			return err
		}
	}

	exposedPort := int32(80)
	if httpso.Spec.Service.Port != 0 {
		exposedPort = httpso.Spec.Service.Port
	}

	serviceType := corev1.ServiceTypeClusterIP
	if httpso.Spec.Service.Type != "" {
		serviceType = httpso.Spec.Service.Type
	}

	// create two services for the interceptor:
	// - for the public proxy
	// - for the admin server (that has the /queue endpoint)
	publicPorts := []corev1.ServicePort{
		k8s.NewTCPServicePort(
			"proxy",
			exposedPort,
			appInfo.InterceptorConfig.ProxyPort,
		),
	}
	publicProxyService := k8s.NewService(
		appInfo.Namespace,
		appInfo.InterceptorProxyServiceName(),
		publicPorts,
		serviceType,
		k8s.Labels(appInfo.InterceptorDeploymentName()),
	)
	adminPorts := []corev1.ServicePort{
		k8s.NewTCPServicePort(
			"admin",
			appInfo.InterceptorConfig.AdminPort,
			appInfo.InterceptorConfig.AdminPort,
		),
	}
	adminService := k8s.NewService(
		appInfo.Namespace,
		appInfo.InterceptorAdminServiceName(),
		adminPorts,
		corev1.ServiceTypeClusterIP,
		k8s.Labels(appInfo.InterceptorDeploymentName()),
	)
	adminErr := cl.Create(ctx, adminService)
	proxyErr := cl.Create(ctx, publicProxyService)
	if adminErr != nil {
		if errors.IsAlreadyExists(adminErr) {
			logger.Info("interceptor admin service already exists, moving on")
		} else {
			logger.Error(adminErr, "Creating interceptor admin service")
			httpso.AddCondition(*v1alpha1.CreateCondition(v1alpha1.Error, metav1.ConditionFalse, v1alpha1.ErrorCreatingInterceptorAdminService).SetMessage(adminErr.Error()))
			return adminErr
		}
	}
	if proxyErr != nil {
		if errors.IsAlreadyExists(adminErr) {
			logger.Info("interceptor proxy service already exists, moving on")
		} else {
			logger.Error(proxyErr, "Creating interceptor proxy service")
			httpso.AddCondition(*v1alpha1.CreateCondition(v1alpha1.Error, metav1.ConditionFalse, v1alpha1.ErrorCreatingInterceptorProxyService).SetMessage(proxyErr.Error()))
			return proxyErr
		}
	}

	httpso.AddCondition(*v1alpha1.CreateCondition(v1alpha1.Created, metav1.ConditionTrue, v1alpha1.InterceptorCreated).SetMessage("Created interceptor"))
	return nil
}
