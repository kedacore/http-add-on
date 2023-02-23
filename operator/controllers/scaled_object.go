package controllers

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	kedav1alpha1 "github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kedacore/http-add-on/operator/api/v1alpha1"
	"github.com/kedacore/http-add-on/pkg/k8s"
)

// createOrUpdateScaledObject attempts to create a new ScaledObject
// according to the given parameters. If the create failed because the
// ScaledObject already exists, attempts to patch the scaledobject.
// otherwise, fails.
func createOrUpdateScaledObject(
	ctx context.Context,
	cl client.Client,
	logger logr.Logger,
	externalScalerHostName string,
	httpso *v1alpha1.HTTPScaledObject,
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
		fmt.Sprintf("%s-app", httpso.GetName()), // HTTPScaledObject name is the same as the ScaledObject name
		httpso.Spec.ScaleTargetRef.Deployment,
		externalScalerHostName,
		httpso.Spec.Host,
		minReplicaCount,
		maxReplicaCount,
		httpso.Spec.CooldownPeriod,
	)

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
			if err := cl.Patch(ctx, appScaledObject, client.Merge); err != nil {
				logger.Error(
					err,
					"failed to patch existing ScaledObject",
				)
				return err
			}
		} else {
			logger.Error(err, "Creating ScaledObject")
			httpso.AddCondition(*v1alpha1.CreateCondition(
				v1alpha1.Error,
				v1.ConditionFalse,
				v1alpha1.ErrorCreatingAppScaledObject,
			).SetMessage(err.Error()))
			return err
		}
	}

	httpso.AddCondition(*v1alpha1.CreateCondition(
		v1alpha1.Created,
		v1.ConditionTrue,
		v1alpha1.AppScaledObjectCreated,
	).SetMessage("App ScaledObject created"))

	return nil
}
