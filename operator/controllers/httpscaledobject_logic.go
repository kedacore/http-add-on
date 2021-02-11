package controllers

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/kedacore/http-add-on/operator/api/v1alpha1"
	"github.com/kedacore/http-add-on/operator/controllers/config"
	"github.com/kedacore/http-add-on/pkg/k8s"
	appsv1 "k8s.io/api/apps/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (rec *HTTPScaledObjectReconciler) removeApplicationResources(
	ctx context.Context,
	logger logr.Logger,
	appInfo config.AppInfo,
	httpso *v1alpha1.HTTPScaledObject,
) error {

	defer httpso.SaveStatus(context.Background(), logger, rec.Client)
	// Set initial statuses
	httpso.AddCondition(*v1alpha1.CreateCondition(v1alpha1.Terminating, v1.ConditionUnknown, v1alpha1.TerminatingResources).SetMessage("Received termination signal"))

	logger = rec.Log.WithValues(
		"reconciler.appObjects",
		"removeObjects",
		"HTTPScaledObject.name",
		appInfo.Name,
		"HTTPScaledObject.namespace",
		appInfo.Namespace,
	)

	// Delete deployments

	// Delete app deployment
	appDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      appInfo.Name,
			Namespace: appInfo.Namespace,
		},
	}
	if err := rec.Client.Delete(ctx, appDeployment); err != nil {
		if apierrs.IsNotFound(err) {
			logger.Info("App deployment not found, moving on")
		} else {
			logger.Error(err, "Deleting deployment")
			httpso.AddCondition(*v1alpha1.CreateCondition(v1alpha1.Error, v1.ConditionFalse, v1alpha1.AppDeploymentTerminationError).SetMessage(err.Error()))
			return err
		}
	}
	httpso.AddCondition(*v1alpha1.CreateCondition(v1alpha1.Terminated, v1.ConditionTrue, v1alpha1.AppDeploymentTerminated))

	// Delete interceptor deployment
	interceptorDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      appInfo.InterceptorDeploymentName(),
			Namespace: appInfo.Namespace,
		},
	}
	if err := rec.Client.Delete(ctx, interceptorDeployment); err != nil {
		if apierrs.IsNotFound(err) {
			logger.Info("Interceptor deployment not found, moving on")
		} else {
			logger.Error(err, "Deleting interceptor deployment")
			httpso.AddCondition(*v1alpha1.CreateCondition(v1alpha1.Error, v1.ConditionFalse, v1alpha1.InterceptorDeploymentTerminationError).SetMessage(err.Error()))
			return err
		}
	}
	httpso.AddCondition(*v1alpha1.CreateCondition(v1alpha1.Terminated, v1.ConditionTrue, v1alpha1.InterceptorDeploymentTerminated))

	// Delete externalscaler deployment
	externalScalerDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      appInfo.ExternalScalerDeploymentName(),
			Namespace: appInfo.Namespace,
		},
	}
	if err := rec.Client.Delete(ctx, externalScalerDeployment); err != nil {
		if apierrs.IsNotFound(err) {
			logger.Info("External scaler not found, moving on")
		} else {
			logger.Error(err, "Deleting external scaler deployment")
			httpso.AddCondition(*v1alpha1.CreateCondition(v1alpha1.Error, v1.ConditionFalse, v1alpha1.ExternalScalerDeploymentTerminationError).SetMessage(err.Error()))
			return err
		}
	}
	httpso.AddCondition(*v1alpha1.CreateCondition(v1alpha1.Terminated, v1.ConditionTrue, v1alpha1.ExternalScalerDeploymentTerminated))

	// Delete Services

	coreCl := rec.K8sCl.CoreV1().Services(appInfo.Namespace)

	// Delete app service
	if err := coreCl.Delete(ctx, appInfo.Name, metav1.DeleteOptions{}); err != nil {
		if apierrs.IsNotFound(err) {
			logger.Info("App service not found, moving on")
		} else {
			logger.Error(err, "Deleting app service")
			httpso.AddCondition(*v1alpha1.CreateCondition(v1alpha1.Error, v1.ConditionFalse, v1alpha1.AppServiceTerminationError).SetMessage(err.Error()))
			return err
		}
	}
	httpso.AddCondition(*v1alpha1.CreateCondition(v1alpha1.Error, v1.ConditionFalse, v1alpha1.AppServiceTerminated))

	// Delete interceptor admin and proxy services
	if err := coreCl.Delete(ctx, appInfo.InterceptorAdminServiceName(), metav1.DeleteOptions{}); err != nil {
		if apierrs.IsNotFound(err) {
			logger.Info("Interceptor admin service not found, moving on")
		} else {
			logger.Error(err, "Deleting interceptor admin service")
			httpso.AddCondition(*v1alpha1.CreateCondition(v1alpha1.Error, v1.ConditionFalse, v1alpha1.InterceptorAdminServiceTerminationError).SetMessage(err.Error()))
			return err
		}
	}
	if err := coreCl.Delete(ctx, appInfo.InterceptorProxyServiceName(), metav1.DeleteOptions{}); err != nil {
		if apierrs.IsNotFound(err) {
			logger.Info("Interceptor proxy service not found, moving on")
		} else {
			logger.Error(err, "Deleting interceptor proxy service")
			httpso.AddCondition(*v1alpha1.CreateCondition(v1alpha1.Error, v1.ConditionFalse, v1alpha1.InterceptorProxyServiceTerminationError).SetMessage(err.Error()))
			return err
		}
	}
	httpso.AddCondition(*v1alpha1.CreateCondition(v1alpha1.Terminated, v1.ConditionTrue, v1alpha1.InterceptorAdminServiceTerminated))
	httpso.AddCondition(*v1alpha1.CreateCondition(v1alpha1.Terminated, v1.ConditionTrue, v1alpha1.InterceptorProxyServiceTerminated))

	// Delete external scaler service
	if err := coreCl.Delete(ctx, appInfo.ExternalScalerServiceName(), metav1.DeleteOptions{}); err != nil {
		if apierrs.IsNotFound(err) {
			logger.Info("External scaler service not found, moving on")
		} else {
			logger.Error(err, "Deleting external scaler service")
			httpso.AddCondition(*v1alpha1.CreateCondition(v1alpha1.Error, v1.ConditionFalse, v1alpha1.ExternalScalerServiceTerminationError).SetMessage(err.Error()))
			return err
		}
	}
	httpso.AddCondition(*v1alpha1.CreateCondition(v1alpha1.Terminated, v1.ConditionTrue, v1alpha1.ExternalScalerServiceTerminated))

	// Delete ScaledObject
	// TODO: use r.Client here, not the dynamic one
	scaledObjectCl := k8s.NewScaledObjectClient(rec.K8sDynamicCl)
	if err := scaledObjectCl.Namespace(appInfo.Namespace).Delete(ctx, appInfo.ScaledObjectName(), metav1.DeleteOptions{}); err != nil {
		if apierrs.IsNotFound(err) {
			logger.Info("App ScaledObject not found, moving on")
		} else {
			logger.Error(err, "Deleting scaledobject")
			httpso.AddCondition(*v1alpha1.CreateCondition(v1alpha1.Error, v1.ConditionFalse, v1alpha1.ScaledObjectTerminationError).SetMessage(err.Error()))
			return err
		}
	}
	httpso.AddCondition(*v1alpha1.CreateCondition(v1alpha1.Terminated, v1.ConditionTrue, v1alpha1.ScaledObjectTerminated))

	return nil
}

func (rec *HTTPScaledObjectReconciler) createOrUpdateApplicationResources(
	ctx context.Context,
	logger logr.Logger,
	appInfo config.AppInfo,
	httpso *v1alpha1.HTTPScaledObject,
) error {
	defer httpso.SaveStatus(context.Background(), logger, rec.Client)
	logger = rec.Log.WithValues(
		"reconciler.appObjects",
		"addObjects",
		"HTTPScaledObject.name",
		appInfo.Name,
		"HTTPScaledObject.namespace",
		appInfo.Namespace,
	)

	// set initial statuses
	httpso.AddCondition(*v1alpha1.CreateCondition(v1alpha1.Pending, v1.ConditionUnknown, v1alpha1.PendingCreation).SetMessage("Identified HTTPScaledObject creation signal"))

	// CREATING THE USER APPLICATION
	if err := createUserApp(ctx, appInfo, rec.Client, logger, httpso); err != nil {
		return err
	}

	// CREATING INTERNAL ADD-ON OBJECTS
	// Creating the dedicated interceptor
	if err := createInterceptor(ctx, appInfo, rec.Client, logger, httpso); err != nil {
		return err
	}

	// create dedicated external scaler for this app
	if err := createExternalScaler(ctx, appInfo, rec.Client, logger, httpso); err != nil {

		return err

	}

	// create the KEDA core ScaledObject (not the HTTP one).
	// this needs to be submitted so that KEDA will scale the app's deployment
	if err := createScaledObject(ctx, appInfo, rec.Client, logger, httpso); err != nil {
		return err

	}

	// TODO: Create a new ingress resource that will point to the interceptor

	return nil
}
