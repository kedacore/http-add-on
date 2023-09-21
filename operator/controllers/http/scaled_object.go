package http

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	kedav1alpha1 "github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	httpv1alpha1 "github.com/kedacore/http-add-on/operator/apis/http/v1alpha1"
	"github.com/kedacore/http-add-on/pkg/k8s"
)

// createOrUpdateScaledObject attempts to create a new ScaledObject
// according to the given parameters. If the create failed because the
// ScaledObject already exists, attempts to patch the scaledobject.
// otherwise, fails.
func (r *HTTPScaledObjectReconciler) createOrUpdateScaledObject(
	ctx context.Context,
	cl client.Client,
	logger logr.Logger,
	externalScalerHostName string,
	patchStrategy string,
	httpso *httpv1alpha1.HTTPScaledObject,
) error {
	logger.Info("Creating scaled objects", "external scaler host name", externalScalerHostName)

	var minReplicaCount *int32
	var maxReplicaCount *int32
	if replicas := httpso.Spec.Replicas; replicas != nil {
		minReplicaCount = replicas.Min
		maxReplicaCount = replicas.Max
	}

	appScaledObject := k8s.NewScaledObject(
		httpso.GetNamespace(),
		httpso.GetName(), // HTTPScaledObject name is the same as the ScaledObject name
		httpso.Spec.ScaleTargetRef.Deployment,
		externalScalerHostName,
		httpso.Spec.Hosts,
		httpso.Spec.PathPrefixes,
		minReplicaCount,
		maxReplicaCount,
		httpso.Spec.CooldownPeriod,
	)

	// Set HTTPScaledObject instance as the owner and controller
	if err := controllerutil.SetControllerReference(httpso, appScaledObject, r.Scheme); err != nil {
		return err
	}

	logger.Info("Creating App ScaledObject", "ScaledObject", *appScaledObject)
	if err := cl.Create(ctx, appScaledObject); err != nil {
		if errors.IsAlreadyExists(err) {
			existingSOKey := client.ObjectKey{
				Namespace: httpso.GetNamespace(),
				Name:      appScaledObject.GetName(),
			}
			var fetchedSO kedav1alpha1.ScaledObject
			if err := cl.Get(ctx, existingSOKey, &fetchedSO); err != nil {
				logger.Error(
					err,
					"failed to fetch existing ScaledObject for patching",
				)
				return err
			}

			var targetSo = appScaledObject
			if patchStrategy == "APPEND" {
				appendOrUpdateTriggers(&fetchedSO, appScaledObject)
				targetSo = &fetchedSO
			}

			if err := cl.Patch(ctx, targetSo, client.Merge); err != nil {
				logger.Error(
					err,
					"failed to patch existing ScaledObject",
				)
				return err
			}
		} else {
			AddCondition(
				httpso,
				*SetMessage(
					CreateCondition(
						httpv1alpha1.Error,
						v1.ConditionFalse,
						httpv1alpha1.ErrorCreatingAppScaledObject,
					),
					err.Error(),
				),
			)

			logger.Error(err, "Creating ScaledObject")
			return err
		}
	}

	AddCondition(
		httpso,
		*SetMessage(
			CreateCondition(
				httpv1alpha1.Created,
				v1.ConditionTrue,
				httpv1alpha1.AppScaledObjectCreated,
			),
			"App ScaledObject created",
		),
	)

	return r.purgeLegacySO(ctx, cl, logger, httpso)
}

func appendOrUpdateTriggers(existingSO *kedav1alpha1.ScaledObject, appScaledObject *kedav1alpha1.ScaledObject) {
	var triggerExists = false
	var httpTrigger = appScaledObject.Spec.Triggers[0]
	for _, trigger := range appScaledObject.Spec.Triggers {
		if httpTrigger.Name == trigger.Name && httpTrigger.Type == trigger.Type {
			triggerExists = true
			trigger = httpTrigger
			break
		}
	}
	if !triggerExists {
		existingSO.Spec.Triggers = append(existingSO.Spec.Triggers, httpTrigger)
	}
}

// TODO(pedrotorres): delete this on v0.6.0
func (r *HTTPScaledObjectReconciler) purgeLegacySO(
	ctx context.Context,
	cl client.Client,
	logger logr.Logger,
	httpso *httpv1alpha1.HTTPScaledObject,
) error {
	legacyName := fmt.Sprintf("%s-app", httpso.GetName())
	legacyKey := client.ObjectKey{
		Namespace: httpso.GetNamespace(),
		Name:      legacyName,
	}

	var legacySO kedav1alpha1.ScaledObject
	if err := cl.Get(ctx, legacyKey, &legacySO); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("legacy ScaledObject not found")
			return nil
		}

		logger.Error(err, "failed getting legacy ScaledObject")
		return err
	}

	if err := cl.Delete(ctx, &legacySO); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("legacy ScaledObject not found")
			return nil
		}

		logger.Error(err, "failed deleting legacy ScaledObject")
		return err
	}

	return nil
}
