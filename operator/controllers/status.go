package controllers

import (
	"context"

	"github.com/go-logr/logr"
	httpsoapi "github.com/kedacore/http-add-on/operator/api/v1alpha1"
	httpv1alpha1 "github.com/kedacore/http-add-on/operator/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func updateStatus(
	ctx context.Context,
	logger logr.Logger,
	cl client.Client,
	httpso *httpv1alpha1.HTTPScaledObject,
) {
	logger.Info("Updating status on HTTPScaledObject", "httpso", *httpso)
	// patch := runtimeclient.MergeFrom(httpso.DeepCopy())
	// err := cl.Status().Patch(ctx, httpso, patch)

	err := cl.Status().Update(ctx, httpso)
	if err != nil {
		logger.Error(err, "failed to update status on HTTPScaledObject")
	}
}

func allReady(httpso *httpv1alpha1.HTTPScaledObject) bool {
	return httpso.Status.DeploymentStatus == httpsoapi.Created &&
		httpso.Status.ScaledObjectStatus == httpsoapi.Created &&
		httpso.Status.InterceptorStatus == httpsoapi.Created &&
		httpso.Status.ExternalScalerStatus == httpsoapi.Created &&
		httpso.Status.ServiceStatus == httpsoapi.Created
}
