package controllers

import (
	"github.com/go-logr/logr"
	"github.com/kedacore/http-add-on/operator/api/v1alpha1"
	"github.com/kedacore/http-add-on/operator/controllers/config"
	"github.com/kedacore/http-add-on/pkg/k8s"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (rec *HTTPScaledObjectReconciler) removeApplicationResources(
	logger logr.Logger,
	appInfo config.AppInfo,
	httpso *v1alpha1.HTTPScaledObject,
	updateStatus func(httpso *v1alpha1.HTTPScaledObject),
) error {

	defer updateStatus(httpso)
	// set initial statuses
	httpso.Status = v1alpha1.HTTPScaledObjectStatus{
		ServiceStatus:        v1alpha1.Terminating,
		DeploymentStatus:     v1alpha1.Terminating,
		ScaledObjectStatus:   v1alpha1.Terminating,
		InterceptorStatus:    v1alpha1.Terminating,
		ExternalScalerStatus: v1alpha1.Terminating,
		Ready:                false,
	}
	logger = rec.Log.WithValues(
		"reconciler.appObjects",
		"removeObjects",
		"HTTPScaledObject.name",
		appInfo.App.Name,
		"HTTPScaledObject.namespace",
		appInfo.App.Namespace,
	)

	// Delete deployments
	appsCl := rec.K8sCl.AppsV1().Deployments(appInfo.App.Namespace)

	// Delete app deployment
	if err := appsCl.Delete(appInfo.App.Name, &metav1.DeleteOptions{}); err != nil {
		if apierrs.IsNotFound(err) {
			logger.Info("App deployment not found, moving on")
		} else {
			logger.Error(err, "Deleting deployment")
			httpso.Status.DeploymentStatus = v1alpha1.Error
			return err
		}
	}
	httpso.Status.DeploymentStatus = v1alpha1.Deleted

	// Delete interceptor deployment
	if err := appsCl.Delete(appInfo.InterceptorDeploymentName(), &metav1.DeleteOptions{}); err != nil {
		if apierrs.IsNotFound(err) {
			logger.Info("Interceptor deployment not found, moving on")
		} else {
			logger.Error(err, "Deleting interceptor deployment")
			httpso.Status.InterceptorStatus = v1alpha1.Error
			return err
		}
	}

	// Delete externalscaler deployment
	if err := appsCl.Delete(appInfo.ExternalScalerDeploymentName(), &metav1.DeleteOptions{}); err != nil {
		if apierrs.IsNotFound(err) {
			logger.Info("External scaler not found, moving on")
		} else {
			logger.Error(err, "Deleting external scaler deployment")
			httpso.Status.ExternalScalerStatus = v1alpha1.Error
			return err
		}
	}

	// Delete Services
	coreCl := rec.K8sCl.CoreV1().Services(appInfo.App.Namespace)

	// Delete app service
	if err := coreCl.Delete(appInfo.App.Name, &metav1.DeleteOptions{}); err != nil {
		if apierrs.IsNotFound(err) {
			logger.Info("App service not found, moving on")
		} else {
			logger.Error(err, "Deleting app service")
			httpso.Status.ServiceStatus = v1alpha1.Error
			return err
		}
	}
	httpso.Status.ServiceStatus = v1alpha1.Deleted

	// Delete interceptor admin and proxy services
	if err := coreCl.Delete(appInfo.InterceptorAdminServiceName(), &metav1.DeleteOptions{}); err != nil {
		if apierrs.IsNotFound(err) {
			logger.Info("Interceptor admin service not found, moving on")
		} else {
			logger.Error(err, "Deleting interceptor admin service")
			httpso.Status.InterceptorStatus = v1alpha1.Error
			return err
		}
	}
	if err := coreCl.Delete(appInfo.InterceptorProxyServiceName(), &metav1.DeleteOptions{}); err != nil {
		if apierrs.IsNotFound(err) {
			logger.Info("Interceptor proxy service not found, moving on")
		} else {
			logger.Error(err, "Deleting interceptor proxy service")
			httpso.Status.InterceptorStatus = v1alpha1.Error
			return err
		}
	}
	httpso.Status.InterceptorStatus = v1alpha1.Deleted

	// Delete external scaler service
	if err := coreCl.Delete(appInfo.ExternalScalerServiceName(), &metav1.DeleteOptions{}); err != nil {
		if apierrs.IsNotFound(err) {
			logger.Info("External scaler service not found, moving on")
		} else {
			logger.Error(err, "Deleting external scaler service")
			httpso.Status.ExternalScalerStatus = v1alpha1.Error
			return err
		}
	}
	httpso.Status.ExternalScalerStatus = v1alpha1.Deleted

	// Delete ScaledObject
	// TODO: use r.Client here, not the dynamic one
	scaledObjectCl := k8s.NewScaledObjectClient(rec.K8sDynamicCl)
	if err := scaledObjectCl.Namespace(appInfo.App.Namespace).Delete(appInfo.ScaledObjectName(), &metav1.DeleteOptions{}); err != nil {
		if apierrs.IsNotFound(err) {
			logger.Info("App ScaledObject not found, moving on")
		} else {
			logger.Error(err, "Deleting scaledobject")
			httpso.Status.ScaledObjectStatus = v1alpha1.Error
			return err
		}
	}
	httpso.Status.ScaledObjectStatus = v1alpha1.Deleted
	return nil
}

func (rec *HTTPScaledObjectReconciler) createOrUpdateApplicationResources(
	logger logr.Logger,
	appInfo config.AppInfo,
	httpso *v1alpha1.HTTPScaledObject,
	updateStatus func(httpso *v1alpha1.HTTPScaledObject),
) error {
	defer updateStatus(httpso)
	logger = rec.Log.WithValues(
		"reconciler.appObjects",
		"addObjects",
		"HTTPScaledObject.name",
		appInfo.App.Name,
		"HTTPScaledObject.namespace",
		appInfo.App.Namespace,
	)

	// set initial statuses
	httpso.Status = v1alpha1.HTTPScaledObjectStatus{
		ServiceStatus:        v1alpha1.Pending,
		DeploymentStatus:     v1alpha1.Pending,
		ScaledObjectStatus:   v1alpha1.Pending,
		InterceptorStatus:    v1alpha1.Pending,
		ExternalScalerStatus: v1alpha1.Pending,
		Ready:                false,
	}

	// CREATING THE USER APPLICATION
	if err := createUserApp(appInfo, rec.K8sCl, logger, httpso); err != nil {
		return err
	}

	// CREATING INTERNAL ADD-ON OBJECTS
	// Creating the dedicated interceptor
	if err := createInterceptor(appInfo, rec.K8sCl, logger, httpso); err != nil {
		return err
	}

	// create dedicated external scaler for this app
	if err := createExternalScaler(appInfo, rec.K8sCl, logger, httpso); err != nil {

		return err

	}

	// create the KEDA core ScaledObject (not the HTTP one).
	// this needs to be submitted so that KEDA will scale the app's deployment
	if err := createScaledObject(appInfo, rec.K8sDynamicCl, logger, httpso); err != nil {
		return err
	}

	return nil
}
