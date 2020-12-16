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

	httpv1alpha1 "github.com/kedacore/http-add-on/operator/api/v1alpha1"
)

// HTTPScaledObjectReconciler reconciles a HTTPScaledObject object
type HTTPScaledObjectReconciler struct {
	K8sCl                 *kubernetes.Clientset
	K8sDynamicCl          dynamic.Interface
	ExternalScalerAddress string
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=http.keda.sh,resources=scaledobjects,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=http.keda.sh,resources=scaledobjects/status,verbs=get;update;patch

// Reconcile reconciles a newly created, deleted, or otherwise changed
// HTTPScaledObject
func (rec *HTTPScaledObjectReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	logger := rec.Log.WithValues("HTTPScaledObject.Namespace", req.Namespace, "HTTPScaledObject.Name", req.Name)

	ctx := context.Background()
	_ = rec.Log.WithValues("httpscaledobject", req.NamespacedName)
	httpso := &httpv1alpha1.HTTPScaledObject{}

	if err := rec.Client.Get(ctx, client.ObjectKey{
		Name:      req.Name,
		Namespace: req.Namespace,
	}, httpso); err != nil {
		if errors.IsNotFound(err) {
			// If the HTTPScaledObject wasn't found, it might have
			// been deleted between the reconcile and the get.
			// It'll automatically get garbage collected, so don't
			// schedule a requeue
			return ctrl.Result{}, nil
		}
		// if we didn't get a not found error, log it and schedule a requeue
		// with a backoff
		logger.Error(err, "Getting the HTTP Scaled obj")
		return ctrl.Result{
			RequeueAfter: 500 * time.Millisecond,
		}, err
	}

	if httpso.GetDeletionTimestamp() != nil {
		// if it was marked deleted, delete all the related objects
		// and don't schedule for another reconcile. Kubernetes
		// will finalize them
		removeErr := rec.removeAppObjects(logger, req, httpso)
		if removeErr != nil {
			logger.Error(removeErr, "Removing application objects")
		}
		return ctrl.Result{}, removeErr
	}

	appName := httpso.Spec.AppName
	image := httpso.Spec.Image
	port := httpso.Spec.Port
	httpso.Status = httpv1alpha1.HTTPScaledObjectStatus{
		ServiceStatus: httpv1alpha1.Unknown,
		DeploymentStatus: httpv1alpha1.Unknown,
		ScaledObjectStatus: httpv1alpha1.Unknown,
		Ready: false,
	}
	logger.Info("App Name: %s, image: %s, port: %d", appName, image, port)

	if err := rec.addAppObjects(logger, req, httpso); err != nil {
		logger.Error(err, "Adding app objects")
		if removeErr := rec.removeAppObjects(logger, req, httpso); removeErr != nil {
			logger.Error(removeErr, "Removing previously created resources")
		}
		return ctrl.Result{}, err
	}

	var pollingInterval int32 = 50000
	if httpso.Spec.PollingInterval != 0 {
		pollingInterval = httpso.Spec.PollingInterval
	}

	return ctrl.Result{
		RequeueAfter: time.Millisecond * time.Duration(pollingInterval),
	}, nil
}

// SetupWithManager starts up reconciliation with the given manager
func (rec *HTTPScaledObjectReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&httpv1alpha1.HTTPScaledObject{}).
		Complete(rec)
}
