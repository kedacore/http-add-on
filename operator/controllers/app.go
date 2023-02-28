package controllers

import (
	"context"

	"github.com/go-logr/logr"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kedacore/http-add-on/operator/api/v1alpha1"
	"github.com/kedacore/http-add-on/operator/controllers/config"
	"github.com/kedacore/http-add-on/pkg/routing"
)

func removeApplicationResources(
	ctx context.Context,
	logger logr.Logger,
	cl client.Client,
	routingTable *routing.Table,
	baseConfig config.Base,
	httpso *v1alpha1.HTTPScaledObject,
) error {
	defer httpso.SaveStatus(context.Background(), logger, cl)
	// Set initial statuses
	httpso.AddCondition(*v1alpha1.CreateCondition(
		v1alpha1.Terminating,
		v1.ConditionUnknown,
		v1alpha1.TerminatingResources,
	).SetMessage("Received termination signal"))

	logger = logger.WithValues(
		"reconciler.appObjects",
		"removeObjects",
		"HTTPScaledObject.name",
		httpso.Name,
		"HTTPScaledObject.namespace",
		httpso.Namespace,
	)

	// Delete App ScaledObject
	scaledObject := &unstructured.Unstructured{}
	scaledObject.SetNamespace(httpso.Namespace)
	scaledObject.SetName(config.AppScaledObjectName(httpso))
	scaledObject.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "keda.sh",
		Kind:    "ScaledObject",
		Version: "v1alpha1",
	})
	if err := cl.Delete(ctx, scaledObject); err != nil {
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
		cl,
		routingTable,
		httpso.Spec.Host,
		baseConfig.CurrentNamespace,
	); err != nil {
		return err
	}

	return nil
}

func createOrUpdateApplicationResources(
	ctx context.Context,
	logger logr.Logger,
	cl client.Client,
	routingTable *routing.Table,
	baseConfig config.Base,
	externalScalerConfig config.ExternalScaler,
	httpso *v1alpha1.HTTPScaledObject,
) error {
	defer httpso.SaveStatus(context.Background(), logger, cl)
	logger = logger.WithValues(
		"reconciler.appObjects",
		"addObjects",
		"HTTPScaledObject.name",
		httpso.Name,
		"HTTPScaledObject.namespace",
		httpso.Namespace,
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
	if err := createOrUpdateScaledObject(
		ctx,
		cl,
		logger,
		externalScalerConfig.HostName(baseConfig.CurrentNamespace),
		httpso,
	); err != nil {
		return err
	}

	targetPendingReqs := baseConfig.TargetPendingRequests
	if tpr := httpso.Spec.TargetPendingRequests; tpr != nil {
		targetPendingReqs = *tpr
	}

	if err := addAndUpdateRoutingTable(
		ctx,
		logger,
		cl,
		routingTable,
		httpso.Spec.Host,
		routing.NewTarget(
			httpso.GetNamespace(),
			httpso.Spec.ScaleTargetRef.Service,
			int(httpso.Spec.ScaleTargetRef.Port),
			httpso.Spec.ScaleTargetRef.Deployment,
			targetPendingReqs,
		),
		baseConfig.CurrentNamespace,
	); err != nil {
		return err
	}
	return nil
}
