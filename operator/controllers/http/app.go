package http

import (
	"context"
	"strings"

	"github.com/go-logr/logr"
	kedav1alpha1 "github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kedacore/http-add-on/operator/apis/http/v1alpha1"
	"github.com/kedacore/http-add-on/operator/controllers/http/config"
)

var SkipScaledObjectCreationAnnotation = "httpscaledobject.keda.sh/skip-scaledobject-creation"

func (r *HTTPScaledObjectReconciler) createOrUpdateApplicationResources(
	ctx context.Context,
	logger logr.Logger,
	cl client.Client,
	baseConfig config.Base,
	externalScalerConfig config.ExternalScaler,
	httpso *v1alpha1.HTTPScaledObject,
) error {
	logger = logger.WithValues(
		"reconciler.appObjects",
		"addObjects",
		"HTTPScaledObject.name",
		httpso.Name,
		"HTTPScaledObject.namespace",
		httpso.Namespace,
	)

	// We want to integrate http scaler with other
	// scalers. when "httpscaledobject.keda.sh/skip-scaledobject-creation" is set to true,
	// reconciler will skip the KEDA core ScaledObjects creation or delete scaledObject if it already exists.
	// you can then create your own SO, and add http scaler as one of your triggers.
	if httpso.Annotations[SkipScaledObjectCreationAnnotation] == "true" {
		logger.Info(
			"Skip scaled objects creation with flag 'httpscaledobject.keda.sh/skip-scaledobject-creation'=true",
			"HTTPScaledObject", httpso.Name)
		err := r.deleteScaledObject(ctx, cl, logger, httpso)
		if err != nil {
			logger.Info("Failed to delete ScaledObject",
				"HTTPScaledObject", httpso.Name)
		}
		return nil
	}

	// create the KEDA core ScaledObjects (not the HTTP one) for
	// the app deployment and the interceptor deployment.
	// this needs to be submitted so that KEDA will scale both the app and
	// interceptor
	return r.createOrUpdateScaledObject(
		ctx,
		cl,
		logger,
		externalScalerConfig.HostName(baseConfig.CurrentNamespace),
		httpso,
	)
}

func (r *HTTPScaledObjectReconciler) deleteScaledObject(
	ctx context.Context,
	cl client.Client,
	logger logr.Logger,
	httpso *v1alpha1.HTTPScaledObject,
) error {
	var fetchedSO kedav1alpha1.ScaledObject

	objectKey := types.NamespacedName{
		Namespace: httpso.Namespace,
		Name:      httpso.Name,
	}

	if err := cl.Get(ctx, objectKey, &fetchedSO); err != nil {
		logger.Info("Failed to retrieve ScaledObject",
			"ScaledObject", &fetchedSO.Name)
		return err
	}

	if isOwnerReferenceMatch(&fetchedSO, httpso) {
		if err := cl.Delete(ctx, &fetchedSO); err != nil {
			logger.Info("Failed to delete ScaledObject",
				"ScaledObject", &fetchedSO.Name)
			return nil
		}
		logger.Info("Deleted ScaledObject",
			"ScaledObject", &fetchedSO.Name)
	}

	return nil
}

// function to check if the owner reference of ScaledObject matches the HTTPScaledObject
func isOwnerReferenceMatch(scaledObject *kedav1alpha1.ScaledObject, httpso *v1alpha1.HTTPScaledObject) bool {
	for _, ownerRef := range scaledObject.OwnerReferences {
		if strings.ToLower(ownerRef.Kind) == "httpscaledobject" &&
			ownerRef.Name == httpso.Name {
			return true
		}
	}
	return false
}
