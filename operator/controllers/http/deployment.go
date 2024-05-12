package http

import (
	"context"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// +kubebuilder:rbac:groups="apps",resources=deployments,verbs=get;list;watch;create;update;patch;delete

func createOrUpdateDeployment(
	ctx context.Context,
	logger logr.Logger,
	cl client.Client,
	deployment *appsv1.Deployment,
) error {
	logger.Info("Creating Deployment", "deployment", deployment.Name)
	if err := cl.Create(ctx, deployment); err != nil {
		if errors.IsAlreadyExists(err) {
			existingServiceKey := client.ObjectKey{
				Namespace: deployment.GetNamespace(),
				Name:      deployment.GetName(),
			}
			if err := cl.Get(ctx, existingServiceKey, &appsv1.Deployment{}); err != nil {
				logger.Error(
					err,
					"failed to fetch existing Deployment for patching",
				)
				return err
			}
			if err := cl.Patch(ctx, deployment, client.Merge); err != nil {
				logger.Error(
					err,
					"failed to patch existing Deployment",
				)
				return err
			}
			return nil
		}
		return err
	}
	return nil
}
