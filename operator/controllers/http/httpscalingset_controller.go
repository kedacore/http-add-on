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

package http

import (
	"context"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	httpv1alpha1 "github.com/kedacore/http-add-on/operator/apis/http/v1alpha1"
	"github.com/kedacore/http-add-on/operator/controllers/util"
)

// HTTPScalingSetReconciler reconciles a HTTPScalingSet object
//
//revive:disable-next-line:exported
//goland:noinspection GoNameStartsWithPackageName
type HTTPScalingSetReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=http.keda.sh,resources=httpscalingsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=http.keda.sh,resources=httpscalingsets/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=http.keda.sh,resources=httpscalingsets/finalizers,verbs=update

// Reconcile reconciles a newly created, deleted, or otherwise changed
// HTTPScaledObject
func (r *HTTPScalingSetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx, "httpscalingset", req.NamespacedName)
	logger.Info("Reconciliation start")

	httpss := &httpv1alpha1.HTTPScalingSet{}
	if err := r.Client.Get(ctx, req.NamespacedName, httpss); err != nil {
		if k8serrors.IsNotFound(err) {
			// If the HTTPScaledObject wasn't found, it might have
			// been deleted between the reconcile and the get.
			// It'll automatically get garbage collected, so don't
			// schedule a requeue
			logger.Info("HTTPScalingSet not found, assuming it was deleted and stopping early")
			return ctrl.Result{}, nil
		}
		// if we didn't get a not found error, log it and schedule a requeue
		// with a backoff
		logger.Error(err, "Getting the HTTP Scaled obj, requeueing")
		return ctrl.Result{
			RequeueAfter: 500 * time.Millisecond,
		}, err
	}

	if httpss.GetDeletionTimestamp() != nil {
		return ctrl.Result{}, finalizeScaledObject(ctx, logger, r.Client, httpss)
	}

	// ensure finalizer is set on this resource
	if err := ensureFinalizer(ctx, logger, r.Client, httpss); err != nil {
		return ctrl.Result{}, err
	}

	// Create required app objects for the application defined by the CRD
	err := createOrUpdateInterceptorResources(
		ctx,
		logger,
		r.Client,
		httpss,
		r.Scheme,
	)
	if err != nil {
		return ctrl.Result{}, err
	}
	err = createOrUpdateExternalScalerResources(
		ctx,
		logger,
		r.Client,
		httpss,
		r.Scheme,
	)
	if err != nil {
		return ctrl.Result{}, err
	}

	// success reconciling
	logger.Info("Reconcile success")
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *HTTPScalingSetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&httpv1alpha1.HTTPScalingSet{}, builder.WithPredicates(
			predicate.Or(
				predicate.GenerationChangedPredicate{},
			),
		)).
		// Trigger a reconcile only when the Deployment spec,label or annotation changes.
		// Ignore updates to Deployment status
		Owns(&appsv1.Deployment{}, builder.WithPredicates(
			predicate.Or(
				predicate.LabelChangedPredicate{},
				predicate.AnnotationChangedPredicate{},
				util.DeploymentSpecChangedPredicate{},
			))).
		// Trigger a reconcile only when the Service spec,label or annotation changes.
		// Ignore updates to Service status
		Owns(&corev1.Service{}, builder.WithPredicates(
			predicate.Or(
				predicate.LabelChangedPredicate{},
				predicate.AnnotationChangedPredicate{},
				util.ServiceSpecChangedPredicate{},
			))).
		Complete(r)
}
