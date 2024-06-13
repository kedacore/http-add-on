package http

import (
	"context"

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
func (r *HTTPScaledObjectReconciler) createOrUpdateScaledObject(ctx context.Context, cl client.Client, logger logr.Logger, externalScalerHostName string, httpso *httpv1alpha1.HTTPScaledObject) error {
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
		httpso.Labels,
		httpso.Annotations,
		httpso.Spec.ScaleTargetRef,
		externalScalerHostName,
		minReplicaCount,
		maxReplicaCount,
		httpso.Spec.CooldownPeriod,
		httpso.ResourceVersion,
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
				logger.Error(err, "failed to fetch existing ScaledObject for patching")
				return err
			}
			if err := cl.Patch(ctx, appScaledObject, client.Merge); err != nil {
				logger.Error(err, "failed to patch existing ScaledObject")
				return err
			}
		} else {
			AddOrUpdateCondition(
				httpso,
				*SetMessage(
					CreateCondition(
						httpv1alpha1.Ready,
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

	AddOrUpdateCondition(
		httpso,
		*SetMessage(
			CreateCondition(
				httpv1alpha1.Ready,
				v1.ConditionTrue,
				httpv1alpha1.AppScaledObjectCreated,
			),
			"App ScaledObject created",
		),
	)
	return nil
}
