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

func createInterceptor(
	appInfo config.AppInfo,
	cl *kubernetes.Clientset,
	logger logr.Logger,
	httpso *v1alpha1.HTTPScaledObject,
) error {
	interceptorEnvs := []corev1.EnvVar{
		{
			Name:  "KEDA_HTTP_APP_SERVICE_NAME",
			Value: appInfo.App.Name,
		},
		{
			Name:  "KEDA_HTTP_APP_SERVICE_PORT",
			Value: fmt.Sprintf("%d", appInfo.App.Port),
		},
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
	deploymentsCl := cl.AppsV1().Deployments(appInfo.App.Namespace)
	if _, err := deploymentsCl.Create(deployment); err != nil {
		if errors.IsAlreadyExists(err) {
			logger.Info("Interceptor deployment already exists, moving on")
		} else {
			logger.Error(err, "Creating interceptor deployment")
			httpso.Status.InterceptorStatus = v1alpha1.Error
			return err
		}
	}

	// create two services for the interceptor:
	// - for the public proxy
	// - for the admin server (that has the /queue endpoint)
	publicPorts := []corev1.ServicePort{
		k8s.NewTCPServicePort(
			"proxy",
			// TODO: make this the public port - probably 80
			80,
			appInfo.InterceptorConfig.ProxyPort,
		),
	}
	publicProxyServiceType := corev1.ServiceTypeLoadBalancer
	if appInfo.App.IngressHost != "" {
		publicProxyServiceType = corev1.ServiceTypeClusterIP
	}
	publicProxyService := k8s.NewService(
		appInfo.InterceptorProxyServiceName(),
		publicPorts,
		publicProxyServiceType,
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
		appInfo.InterceptorAdminServiceName(),
		adminPorts,
		corev1.ServiceTypeClusterIP,
		k8s.Labels(appInfo.InterceptorDeploymentName()),
	)
	servicesCl := cl.CoreV1().Services(appInfo.App.Namespace)
	_, adminErr := servicesCl.Create(adminService)
	_, proxyErr := servicesCl.Create(publicProxyService)
	if adminErr != nil {
		if errors.IsAlreadyExists(adminErr) {
			logger.Info("interceptor admin service already exists, moving on")
		} else {
			logger.Error(adminErr, "Creating interceptor admin service")
			httpso.Status.InterceptorStatus = v1alpha1.Error
			return adminErr
		}
	}
	if proxyErr != nil {
		if errors.IsAlreadyExists(adminErr) {
			logger.Info("interceptor proxy service already exists, moving on")
		} else {
			logger.Error(adminErr, "Creating interceptor proxy service")
			httpso.Status.InterceptorStatus = v1alpha1.Error
			return proxyErr
		}
	}

	// if the app is supposed to use an ingress, then the public proxy service was created
	// above with type ClusterIP and we need to create an Ingress resource for it here
	ingress := k8s.NewIngress(
		appInfo.App.Namespace,
		appInfo.InterceptorIngressName(),
		appInfo.App.IngressHost,
		appInfo.InterceptorProxyServiceName(),
		appInfo.InterceptorConfig.ProxyPort,
	)
	ingressCl := cl.NetworkingV1beta1().Ingresses(appInfo.App.Namespace)
	_, ingressErr := ingressCl.Create(ingress)
	if ingressErr != nil {
		if errors.IsAlreadyExists(ingressErr) {
			logger.Info("interceptor ingress already exists, moving on")
		} else {
			logger.Error(ingressErr, "Creating interceptor ingress")
			httpso.Status.InterceptorStatus = v1alpha1.Error
			return ingressErr
		}
	}

	httpso.Status.InterceptorStatus = v1alpha1.Created
	return nil
}
