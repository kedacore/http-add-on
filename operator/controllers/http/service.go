package http

import (
	"context"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete

func createOrUpdateService(
	ctx context.Context,
	logger logr.Logger,
	cl client.Client,
	service *corev1.Service,
) error {
	logger.Info("Creating Service", "service", service.Name)
	if err := cl.Create(ctx, service); err != nil {
		if errors.IsAlreadyExists(err) {
			existingServiceKey := client.ObjectKey{
				Namespace: service.GetNamespace(),
				Name:      service.GetName(),
			}
			if err := cl.Get(ctx, existingServiceKey, &corev1.Service{}); err != nil {
				logger.Error(
					err,
					"failed to fetch existing Service for patching",
				)
				return err
			}
			if err := cl.Patch(ctx, service, client.Merge); err != nil {
				logger.Error(
					err,
					"failed to patch existing Service",
				)
				return err
			}
		}
	}
	return nil
}
