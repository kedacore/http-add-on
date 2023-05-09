package http

import (
	"context"

	"github.com/go-logr/logr"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kedacore/http-add-on/operator/apis/http/v1alpha1"
	"github.com/kedacore/http-add-on/operator/controllers/http/config"
)

func removeApplicationResources(
	ctx context.Context,
	logger logr.Logger,
	cl client.Client,
	httpso *v1alpha1.HTTPScaledObject,
) error {
	defer SaveStatus(context.Background(), logger, cl, httpso)
	// Set initial statuses
	AddCondition(
		httpso,
		*SetMessage(
			CreateCondition(
				v1alpha1.Terminating,
				v1.ConditionUnknown,
				v1alpha1.TerminatingResources,
			),
			"Received termination signal",
		),
	)

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
	scaledObject.SetName(httpso.Name)
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
			AddCondition(
				httpso,
				*SetMessage(
					CreateCondition(
						v1alpha1.Error,
						v1.ConditionFalse,
						v1alpha1.AppScaledObjectTerminationError,
					),
					err.Error(),
				),
			)
			return err
		}
	}
	AddCondition(httpso, *CreateCondition(
		v1alpha1.Terminated,
		v1.ConditionTrue,
		v1alpha1.AppScaledObjectTerminated,
	))

	return nil
}

func createOrUpdateApplicationResources(
	ctx context.Context,
	logger logr.Logger,
	cl client.Client,
	baseConfig config.Base,
	externalScalerConfig config.ExternalScaler,
	httpso *v1alpha1.HTTPScaledObject,
) error {
	defer SaveStatus(context.Background(), logger, cl, httpso)
	logger = logger.WithValues(
		"reconciler.appObjects",
		"addObjects",
		"HTTPScaledObject.name",
		httpso.Name,
		"HTTPScaledObject.namespace",
		httpso.Namespace,
	)

	// set initial statuses
	AddCondition(
		httpso,
		*SetMessage(
			CreateCondition(
				v1alpha1.Pending,
				v1.ConditionUnknown,
				v1alpha1.PendingCreation,
			),
			"Identified HTTPScaledObject creation signal"),
	)

	// create the KEDA core ScaledObjects (not the HTTP one) for
	// the app deployment and the interceptor deployment.
	// this needs to be submitted so that KEDA will scale both the app and
	// interceptor
	return createOrUpdateScaledObject(
		ctx,
		cl,
		logger,
		externalScalerConfig.HostName(baseConfig.CurrentNamespace),
		httpso,
	)
}
