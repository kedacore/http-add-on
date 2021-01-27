package controllers

import (
	"context"

	"github.com/go-logr/logr"
	httpsoapi "github.com/kedacore/http-add-on/operator/api/v1alpha1"
	httpv1alpha1 "github.com/kedacore/http-add-on/operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// func updateStatus(
// 	ctx context.Context,
// 	logger logr.Logger,
// 	cl client.Client,
// 	object interface{}, //*httpv1alpha1.HTTPScaledObject,
// ) {
// 	logger.Info("Updating status on object", "object", object)
// 	runtimeObj := object.(runtime.Object)
// 	httpso := object.(*v1alpha1.HTTPScaledObject)
// 	patch := runtimeclient.MergeFrom(httpso.DeepCopy())
// 	err := cl.Status().Patch(ctx, runtimeObj, patch)

// 	// err := cl.Status().Update(ctx, httpso)
// 	if err != nil {
// 		logger.Error(err, "failed to update status on HTTPScaledObject", "httpso", httpso)
// 	} else {
// 		logger.Info("Updated status on HTTPScaledObject", "HTTPScaledObject", *httpso)
// 	}
// }

func updateStatus(
	ctx context.Context,
	logger logr.Logger,
	cl client.Client,
	httpso *httpv1alpha1.HTTPScaledObject,
) {
	logger.Info("Updating status on object", "scaledobject", *httpso)

	tmpHTTPSo := &httpv1alpha1.HTTPScaledObject{}
	if err := cl.Get(ctx, types.NamespacedName{
		Namespace: "kedahttp",
		Name:      "myapp",
	}, tmpHTTPSo); err != nil {
		logger.Error(err, "HTTPScaledObject not found on test check")
	} else {
		logger.Info("Found HTTPScaledObject on test check", "scaled object", *tmpHTTPSo)
	}

	tmpHTTPSo.Status = httpso.Status

	// patch := runtimeclient.MergeFrom(httpso.DeepCopy())
	// truePtr := true
	// var statusCl client.StatusWriter
	// statusCl = cl.Status()
	// err := statusCl.Patch(ctx, httpso, patch, &client.PatchOptions{
	// 	Force: &truePtr,
	// })
	var runtimeObj runtime.Object
	runtimeObj = tmpHTTPSo
	err := cl.Status().Update(ctx, runtimeObj)
	httpso = tmpHTTPSo
	// httpso is updated!

	if err != nil {
		logger.Error(err, "failed to update status on HTTPScaledObject", "httpso", httpso)
	} else {
		logger.Info("Updated status on HTTPScaledObject", "HTTPScaledObject", *httpso)
	}
}

func allReady(httpso *httpv1alpha1.HTTPScaledObject) bool {
	return httpso.Status.DeploymentStatus == httpsoapi.Created &&
		httpso.Status.ScaledObjectStatus == httpsoapi.Created &&
		httpso.Status.InterceptorStatus == httpsoapi.Created &&
		httpso.Status.ExternalScalerStatus == httpsoapi.Created &&
		httpso.Status.ServiceStatus == httpsoapi.Created
}
