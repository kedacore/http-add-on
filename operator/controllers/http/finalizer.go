package http

import (
	"context"

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
	if !contains(httpso.GetFinalizers(), httpScaledObjectFinalizer) {
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
	httpso *httpv1alpha1.HTTPScaledObject) error {
	if contains(httpso.GetFinalizers(), httpScaledObjectFinalizer) {
		httpso.SetFinalizers(remove(httpso.GetFinalizers(), httpScaledObjectFinalizer))
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

// contains checks if the passed string is present in the given slice of strings.
// This is taken from github.com/kedacore/keda
func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// remove deletes the passed string from the given slice of strings.
// This is taken from github.com/kedacore/keda
func remove(list []string, s string) []string {
	for i, v := range list {
		if v == s {
			list = append(list[:i], list[i+1:]...)
		}
	}
	return list
}
