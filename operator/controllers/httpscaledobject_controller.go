/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kedacore/http-add-on/operator/api/v1alpha1"
	httpv1alpha1 "github.com/kedacore/http-add-on/operator/api/v1alpha1"
	"github.com/kedacore/http-add-on/operator/controllers/config"
)

// HTTPScaledObjectReconciler reconciles a HTTPScaledObject object
type HTTPScaledObjectReconciler struct {
	K8sCl        *kubernetes.Clientset
	K8sDynamicCl dynamic.Interface
	client.Client
	Log                  logr.Logger
	Scheme               *runtime.Scheme
	InterceptorConfig    config.Interceptor
	ExternalScalerConfig config.ExternalScaler
}

// +kubebuilder:rbac:groups=http.keda.sh,resources=scaledobjects,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=http.keda.sh,resources=scaledobjects/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=pods;services,verbs=get;list;watch;create;delete
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;delete
// +kubebuilder:rbac:groups=networking,resources=ingresses,verbs=get;list;watch;create;delete

// Reconcile reconciles a newly created, deleted, or otherwise changed
// HTTPScaledObject
func (rec *HTTPScaledObjectReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	logger := rec.Log.WithValues("HTTPScaledObject.Namespace", req.Namespace, "HTTPScaledObject.Name", req.Name)

	ctx := context.Background()
	_ = rec.Log.WithValues("httpscaledobject", req.NamespacedName)
	httpso := &httpv1alpha1.HTTPScaledObject{}

	if err := rec.Client.Get(ctx, req.NamespacedName, httpso); err != nil {
		if errors.IsNotFound(err) {
			// If the HTTPScaledObject wasn't found, it might have
			// been deleted between the reconcile and the get.
			// It'll automatically get garbage collected, so don't
			// schedule a requeue
			logger.Info("HTTPScaledObject not found, assuming it was deleted and stopping early")
			return ctrl.Result{}, nil
		}
		// if we didn't get a not found error, log it and schedule a requeue
		// with a backoff
		logger.Error(err, "Getting the HTTP Scaled obj, requeueing")
		return ctrl.Result{
			RequeueAfter: 500 * time.Millisecond,
		}, err
	}

	appName := httpso.Spec.AppName
	image := httpso.Spec.Image
	port := httpso.Spec.Port

	appInfo := config.AppInfo{
		Name:                 appName,
		Port:                 port,
		Image:                image,
		Namespace:            req.Namespace,
		InterceptorConfig:    rec.InterceptorConfig,
		ExternalScalerConfig: rec.ExternalScalerConfig,
	}

	if httpso.GetDeletionTimestamp() != nil {
		// if it was marked deleted, delete all the related objects
		// and don't schedule for another reconcile. Kubernetes
		// will finalize them
		// TODO: move this function call into `finalizeScaledObject`
		removeErr := rec.removeApplicationResources(logger, appInfo, httpso, func(httpso *v1alpha1.HTTPScaledObject) {
			updateStatus(ctx, logger, rec.Client, httpso)
		})
		if removeErr != nil {
			// if we failed to remove app resources, reschedule a reconcile so we can try
			// again
			logger.Error(removeErr, "Removing application objects")
			return ctrl.Result{
				RequeueAfter: 1000 * time.Millisecond,
			}, removeErr
		}
		// after we've deleted app objects, we can finalize
		return ctrl.Result{}, finalizeScaledObject(ctx, logger, rec.Client, httpso)
	}

	// ensure finalizer is set on this resource
	if err := ensureFinalizer(ctx, logger, rec.Client, httpso); err != nil {
		return ctrl.Result{}, err
	}

	// initializes the required variables and set the initial status to unknown
	httpso.Status = httpv1alpha1.HTTPScaledObjectStatus{
		ServiceStatus:        httpv1alpha1.Unknown,
		DeploymentStatus:     httpv1alpha1.Unknown,
		ScaledObjectStatus:   httpv1alpha1.Unknown,
		ExternalScalerStatus: httpv1alpha1.Unknown,
		InterceptorStatus:    httpv1alpha1.Unknown,
		Ready:                false,
	}
	updateStatus(ctx, logger, rec.Client, httpso)

	logger.Info("Reconciling HTTPScaledObject", "Namespace", req.Namespace, "App Name", appName, "image", image, "port", port)

	// Create required app objects for the application defined by the CRD
	if err := rec.createOrUpdateApplicationResources(
		logger,
		appInfo,
		httpso,
		func(httpso *v1alpha1.HTTPScaledObject) {
			updateStatus(ctx, logger, rec.Client, httpso)
		},
	); err != nil {
		// if we failed to create app resources, remove what we've created and exit
		logger.Error(err, "Adding app resources")
		if removeErr := rec.removeApplicationResources(
			logger,
			appInfo,
			httpso,
			func(httpso *v1alpha1.HTTPScaledObject) {
				updateStatus(ctx, logger, rec.Client, httpso)
			}); removeErr != nil {
			logger.Error(removeErr, "Removing previously created resources")
		}

		return ctrl.Result{}, err
	}

	// If all goes well, set the ready status to true
	if allReady(httpso) {
		httpso.Status.Ready = true
	}
	updateStatus(ctx, logger, rec.Client, httpso)

	// success reconciling
	logger.Info("Reconcile success")
	return ctrl.Result{}, nil
}

// SetupWithManager starts up reconciliation with the given manager
func (rec *HTTPScaledObjectReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&httpv1alpha1.HTTPScaledObject{}).
		Complete(rec)
}
