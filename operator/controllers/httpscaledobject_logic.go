package controllers

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/kedacore/http-add-on/operator/api/v1alpha1"
	"github.com/kedacore/http-add-on/operator/controllers/config"
	"github.com/kedacore/http-add-on/pkg/routing"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func (rec *HTTPScaledObjectReconciler) removeApplicationResources(
	ctx context.Context,
	logger logr.Logger,
	appInfo config.AppInfo,
	httpso *v1alpha1.HTTPScaledObject,
) error {

	defer httpso.SaveStatus(context.Background(), logger, rec.Client)
	// Set initial statuses
	httpso.AddCondition(*v1alpha1.CreateCondition(
		v1alpha1.Terminating,
		v1.ConditionUnknown,
		v1alpha1.TerminatingResources,
	).SetMessage("Received termination signal"))

	logger = rec.Log.WithValues(
		"reconciler.appObjects",
		"removeObjects",
		"HTTPScaledObject.name",
		appInfo.Name,
		"HTTPScaledObject.namespace",
		appInfo.Namespace,
	)

	// Delete App ScaledObject
	scaledObject := &unstructured.Unstructured{}
	scaledObject.SetNamespace(appInfo.Namespace)
	scaledObject.SetName(config.AppScaledObjectName(httpso))
	scaledObject.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "keda.sh",
		Kind:    "ScaledObject",
		Version: "v1alpha1",
	})
	if err := rec.Client.Delete(ctx, scaledObject); err != nil {
		if apierrs.IsNotFound(err) {
			logger.Info("App ScaledObject not found, moving on")
		} else {
			logger.Error(err, "Deleting scaledobject")
			httpso.AddCondition(*v1alpha1.CreateCondition(
				v1alpha1.Error,
				v1.ConditionFalse,
				v1alpha1.AppScaledObjectTerminationError,
			).SetMessage(err.Error()))
			return err
		}
	}
	httpso.AddCondition(*v1alpha1.CreateCondition(
		v1alpha1.Terminated,
		v1.ConditionTrue,
		v1alpha1.AppScaledObjectTerminated,
	))

	if err := removeAndUpdateRoutingTable(
		ctx,
		logger,
		rec.Client,
		rec.RoutingTable,
		httpso.Spec.Host,
		httpso.ObjectMeta.Namespace,
	); err != nil {
		return err
	}

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
	httpso.AddCondition(*v1alpha1.CreateCondition(
		v1alpha1.Pending,
		v1.ConditionUnknown,
		v1alpha1.PendingCreation,
	).SetMessage("Identified HTTPScaledObject creation signal"))

	// create the KEDA core ScaledObjects (not the HTTP one) for
	// the app deployment and the interceptor deployment.
	// this needs to be submitted so that KEDA will scale both the app and
	// interceptor
	if err := createScaledObjects(
		ctx,
		appInfo,
		rec.Client,
		logger,
		appInfo.ExternalScalerConfig.HostName(appInfo.Namespace),
		httpso,
	); err != nil {
		return err
	}

	targetPendingReqs := httpso.Spec.TargetPendingRequests
	if targetPendingReqs == 0 {
		targetPendingReqs = rec.BaseConfig.TargetPendingRequests
	}

	if err := addAndUpdateRoutingTable(
		ctx,
		logger,
		rec.Client,
		rec.RoutingTable,
		httpso.Spec.Host,
		routing.NewTarget(
			httpso.Spec.ScaleTargetRef.Service,
			int(httpso.Spec.ScaleTargetRef.Port),
			httpso.Spec.ScaleTargetRef.Deployment,
			targetPendingReqs,
		),
		httpso.ObjectMeta.Namespace,
	); err != nil {
		return err
	}
	return nil
}
