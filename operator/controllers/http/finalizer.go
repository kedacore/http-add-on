package http

import (
	"context"
	"slices"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	httpv1alpha1 "github.com/kedacore/http-add-on/operator/apis/http/v1alpha1"
)

const (
	httpScaledObjectFinalizer = "httpscaledobject.http.keda.sh"
)

// ensureFinalizer check there is finalizer present on the ScaledObject, if not it adds one
func ensureFinalizer(
	ctx context.Context,
	logger logr.Logger,
	client client.Client,
	httpso *httpv1alpha1.HTTPScaledObject,
) error {
	if !slices.Contains(httpso.GetFinalizers(), httpScaledObjectFinalizer) {
		logger.Info("Adding Finalizer for the ScaledObject")
		httpso.SetFinalizers(append(httpso.GetFinalizers(), httpScaledObjectFinalizer))

		// Update CR
		err := client.Update(ctx, httpso)
		if err != nil {
			logger.Error(
				err,
				"Failed to update HTTPScaledObject with a finalizer",
				"finalizer",
				httpScaledObjectFinalizer,
			)
			return err
		}
	}
	return nil
}

func finalizeScaledObject(
	ctx context.Context,
	logger logr.Logger,
	client client.Client,
	httpso *httpv1alpha1.HTTPScaledObject,
) error {
	if slices.Contains(httpso.GetFinalizers(), httpScaledObjectFinalizer) {
		httpso.SetFinalizers(slices.DeleteFunc(httpso.GetFinalizers(), func(s string) bool {
			return s == httpScaledObjectFinalizer
		}))
		if err := client.Update(ctx, httpso); err != nil {
			logger.Error(
				err,
				"Failed to update ScaledObject after removing a finalizer",
				"finalizer",
				httpScaledObjectFinalizer,
			)
			return err
		}
	}

	logger.Info("Successfully finalized HTTPScaledObject")
	return nil
}
