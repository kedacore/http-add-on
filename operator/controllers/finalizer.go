package controllers

import (
	"context"

	"github.com/go-logr/logr"
	httpv1alpha1 "github.com/kedacore/http-add-on/operator/api/v1alpha1"
	"github.com/kedacore/keda/v2/controllers/util"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	httpScaledObjectFinalizer = "httpscaledobject.http.keda.sh"
)

// ensureFinalizer check there is finalizer present on the ScaledObject, if not it adds one
func ensureFinalizer(ctx context.Context, logger logr.Logger, client client.Client, httpso *httpv1alpha1.ScaledObject) error {
	if !util.Contains(httpso.GetFinalizers(), httpScaledObjectFinalizer) {
		logger.Info("Adding Finalizer for the ScaledObject")
		httpso.SetFinalizers(append(httpso.GetFinalizers(), scaledObjectFinalizer))

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
