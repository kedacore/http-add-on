package http

import (
	"context"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// This finalizer is deprecated and we shouldn't use it
	httpScaledObjectFinalizer = "httpscaledobject.http.keda.sh"
	httpFinalizer             = "http.keda.sh"
)

// ensureFinalizer check there is finalizer present on the HTTP resources, if not it adds one
func ensureFinalizer(
	ctx context.Context,
	logger logr.Logger,
	client client.Client,
	httpObject client.Object,
) error {
	if !contains(httpObject.GetFinalizers(), httpFinalizer) {
		logger.Info("Adding Finalizer")
		// We have to ensure that the old finalizer is removed
		// We can remove this code in future versions, like v0.10.0 or later
		finalizers := remove(httpObject.GetFinalizers(), httpScaledObjectFinalizer)

		httpObject.SetFinalizers(append(finalizers, httpFinalizer))

		// Update CR
		err := client.Update(ctx, httpObject)
		if err != nil {
			logger.Error(
				err,
				"Failed to update with a finalizer",
				"name",
				httpObject.GetName(),
				"kind",
				httpObject.GetObjectKind().GroupVersionKind().String(),
				"finalizer",
				httpFinalizer,
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
	httpObject client.Object) error {
	if contains(httpObject.GetFinalizers(), httpFinalizer) {
		httpObject.SetFinalizers(remove(httpObject.GetFinalizers(), httpFinalizer))
		if err := client.Update(ctx, httpObject); err != nil {
			logger.Error(
				err,
				"Failed to update ScaledObject after removing a finalizer",
				"name",
				httpObject.GetName(),
				"kind",
				httpObject.GetObjectKind().GroupVersionKind().String(),
				"finalizer",
				httpFinalizer,
			)
			return err
		}
	}

	logger.Info("Successfully finalized")
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
