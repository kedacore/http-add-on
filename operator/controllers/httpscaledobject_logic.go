package controllers

import (
	"fmt"

	"github.com/go-logr/logr"
	"github.com/kedacore/http-add-on/operator/api/v1alpha1"
	"github.com/kedacore/http-add-on/pkg/k8s"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	typedAppsv1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	typedCorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	// The image registry to be used to download KEDA assets, it'll
	// be concatenated with the image names
	imageRegistry string = "khaosdoctor/"
	// Image name for the interceptor container
	interceptorImageName string = "keda-http-interceptor"
	// Image name for the external scaler container
	externalScalerImageName string = "keda-http-external-scaler"
	// The default port all internal service will expose and work on
	defaultExposedPort int = 8080
)

type userApplicationInfo struct {
	name string
	port int32
	image string
	namespace string
	interceptorName string
	externalScalerName string
}

type kubernetesClients struct {
	appsCl typedAppsv1.DeploymentInterface
	coreCl typedCorev1.ServiceInterface
}

func (rec *HTTPScaledObjectReconciler) removeApplicationResources(
	logger logr.Logger,
	req ctrl.Request,
	httpso *v1alpha1.HTTPScaledObject,
) error {
	appName := httpso.Spec.AppName
	interceptorName := fmt.Sprintf("%s-interceptor", appName)
	externalScalerName := fmt.Sprintf("%s-ext-scaler", appName)

	// set initial statuses
	httpso.Status = v1alpha1.HTTPScaledObjectStatus{
		ServiceStatus:      v1alpha1.Terminating,
		DeploymentStatus:   v1alpha1.Terminating,
		ScaledObjectStatus: v1alpha1.Terminating,
		InterceptorStatus: v1alpha1.Terminating,
		ExternalScalerStatus: v1alpha1.Terminating,
		Ready:              false,
	}
	logger = rec.Log.WithValues("reconciler.appObjects", "removeObjects", "HTTPScaledObject.name", appName, "HTTPScaledObject.namespace", httpso.Namespace)

	// Delete deploys
	appsCl := rec.K8sCl.AppsV1().Deployments(req.Namespace)
	if err := appsCl.Delete(appName, &metav1.DeleteOptions{}); err != nil {
		logger.Error(err, "Deleting deployment")
		httpso.Status.DeploymentStatus = v1alpha1.Error
		return err
	}
	httpso.Status.DeploymentStatus = v1alpha1.Deleted

	if err := appsCl.Delete(interceptorName, &metav1.DeleteOptions{}); err != nil {
		logger.Error(err, "Deleting interceptor deployment")
		httpso.Status.InterceptorStatus = v1alpha1.Error
		return err
	}

	if err := appsCl.Delete(externalScalerName, &metav1.DeleteOptions{}); err != nil {
		logger.Error(err, "Deleting external scaler deployment")
		httpso.Status.ExternalScalerStatus = v1alpha1.Error
		return err
	}

	// Delete Services
	coreCl := rec.K8sCl.CoreV1().Services(req.Namespace)
	if err := coreCl.Delete(appName, &metav1.DeleteOptions{}); err != nil {
		logger.Error(err, "Deleting app service")
		httpso.Status.ServiceStatus = v1alpha1.Error
		return err
	}
	httpso.Status.ServiceStatus = v1alpha1.Deleted

	if err := coreCl.Delete(interceptorName, &metav1.DeleteOptions{}); err != nil {
		logger.Error(err, "Deleting interceptor service")
		httpso.Status.InterceptorStatus = v1alpha1.Error
		return err
	}
	httpso.Status.InterceptorStatus = v1alpha1.Deleted

	if err := coreCl.Delete(externalScalerName, &metav1.DeleteOptions{}); err != nil {
		logger.Error(err, "Deleting external scaler service")
		httpso.Status.ExternalScalerStatus = v1alpha1.Error
		return err
	}
	httpso.Status.ExternalScalerStatus = v1alpha1.Deleted

	// TODO: use r.Client here, not the dynamic one
	scaledObjectCl := k8s.NewScaledObjectClient(rec.K8sDynamicCl)
	if err := scaledObjectCl.Namespace(req.Namespace).Delete(appName, &metav1.DeleteOptions{}); err != nil {
		logger.Error(err, "Deleting scaledobject")
		httpso.Status.ScaledObjectStatus = v1alpha1.Error
		return err
	}
	httpso.Status.ScaledObjectStatus = v1alpha1.Deleted
	return nil
}

func (rec *HTTPScaledObjectReconciler) createApplicationResources(
	logger logr.Logger,
	req ctrl.Request,
	httpso *v1alpha1.HTTPScaledObject,
) error {
	appInfo := userApplicationInfo{
		name: httpso.Spec.AppName,
		image: httpso.Spec.Image,
		port: httpso.Spec.Port,
		namespace: httpso.Namespace,
		interceptorName: httpso.Spec.AppName + "-interceptor",
		externalScalerName: httpso.Spec.AppName + "-ext-scaler",
	}
	logger = rec.Log.WithValues("reconciler.appObjects", "addObjects", "HTTPScaledObject.name", appInfo.name, "HTTPScaledObject.namespace", appInfo.namespace)

	// set initial statuses
	httpso.Status = v1alpha1.HTTPScaledObjectStatus{
		ServiceStatus:      v1alpha1.Pending,
		DeploymentStatus:   v1alpha1.Pending,
		ScaledObjectStatus: v1alpha1.Pending,
		InterceptorStatus: v1alpha1.Pending,
		ExternalScalerStatus: v1alpha1.Pending,
		Ready:              false,
	}

	// Init K8s clients
	k8sClients := kubernetesClients{
		appsCl: rec.K8sCl.AppsV1().Deployments(appInfo.namespace),
		coreCl: rec.K8sCl.CoreV1().Services(appInfo.namespace),
	}

	// CREATING THE USER APPLICATION
	if err := createUserApp(appInfo, k8sClients, logger, httpso); err != nil {
		return err
	}

	// CREATING INTERNAL ADD-ON OBJECTS
	// Creating the dedicated interceptor
	if err := createInterceptor(appInfo, k8sClients, logger, httpso); err != nil {
		return err
	}

	// create dedicated external scaler for this app
	if err := createExternalScaler(appInfo, k8sClients, logger, httpso); err != nil {
		return err
	}

	// create the KEDA core ScaledObject (not the HTTP one).
	// this needs to be submitted so that KEDA will scale the app's deployment
	if err := createScaledObject(appInfo, rec.K8sDynamicCl, logger, httpso); err != nil {
		return err
	}

	// TODO: Create a new ingress resource that will point to the interceptor

	return nil
}
