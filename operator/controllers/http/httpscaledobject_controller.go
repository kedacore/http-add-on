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
	"fmt"
	"time"

	kedav1alpha1 "github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	httpv1alpha1 "github.com/kedacore/http-add-on/operator/apis/http/v1alpha1"
	"github.com/kedacore/http-add-on/operator/controllers/http/config"
	"github.com/kedacore/http-add-on/operator/controllers/util"
)

// HTTPScaledObjectReconciler reconciles a HTTPScaledObject object
//
//revive:disable-next-line:exported
//goland:noinspection GoNameStartsWithPackageName
type HTTPScaledObjectReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	ExternalScalerConfig config.ExternalScaler
	BaseConfig           config.Base
}

// +kubebuilder:rbac:groups=http.keda.sh,resources=httpscaledobjects,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=http.keda.sh,resources=httpscaledobjects/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=http.keda.sh,resources=httpscaledobjects/finalizers,verbs=update
// +kubebuilder:rbac:groups=keda.sh,resources=scaledobjects,verbs=get;list;watch;create;update;patch;delete

// Reconcile reconciles a newly created, deleted, or otherwise changed
// HTTPScaledObject
func (r *HTTPScaledObjectReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx, "httpscaledobject", req.NamespacedName)
	logger.Info("Reconciliation start")

	httpso := &httpv1alpha1.HTTPScaledObject{}
	if err := r.Get(ctx, req.NamespacedName, httpso); err != nil {
		if k8serrors.IsNotFound(err) {
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

	if httpso.GetDeletionTimestamp() != nil {
		return ctrl.Result{}, finalizeScaledObject(ctx, logger, r.Client, httpso)
	}

	// ensure finalizer is set on this resource
	if err := ensureFinalizer(ctx, logger, r.Client, httpso); err != nil {
		return ctrl.Result{}, err
	}

	// update status
	httpso.Status.TargetWorkload = fmt.Sprintf("%s/%s/%s", httpso.Spec.ScaleTargetRef.APIVersion, httpso.Spec.ScaleTargetRef.Kind, httpso.Spec.ScaleTargetRef.Name)
	httpso.Status.TargetService = fmt.Sprintf("%s:%d", httpso.Spec.ScaleTargetRef.Service, httpso.Spec.ScaleTargetRef.Port)

	// httpso is updated now
	logger.Info(
		"Reconciling HTTPScaledObject",
		"Namespace",
		req.Namespace,
		"DeploymentName",
		httpso.Name,
	)

	// check if ScalingMetric is correct
	if httpso.Spec.ScalingMetric != nil &&
		httpso.Spec.ScalingMetric.Concurrency != nil &&
		httpso.Spec.ScalingMetric.Rate != nil {
		err := fmt.Errorf(".spec.scalingMetric.concurrency and .spec.scalingMetric.rate are mutually exclusive")
		logger.Error(err, "invalid values for .spec.scalingMetric")
		return ctrl.Result{}, err
	}

	// Create required app objects for the application defined by the CRD
	err := r.createOrUpdateApplicationResources(
		ctx,
		logger,
		r.Client,
		r.BaseConfig,
		r.ExternalScalerConfig,
		httpso,
	)
	if err != nil {
		return ctrl.Result{}, err
	}

	SaveStatus(
		ctx,
		logger,
		r.Client,
		AddOrUpdateCondition(
			httpso,
			*SetMessage(
				CreateCondition(
					httpv1alpha1.Ready,
					v1.ConditionTrue,
					httpv1alpha1.HTTPScaledObjectIsReady,
				),
				"Finished object creation",
			),
		),
	)

	// success reconciling
	logger.Info("Reconcile success")
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *HTTPScaledObjectReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&httpv1alpha1.HTTPScaledObject{}, builder.WithPredicates(
			predicate.Or(
				predicate.GenerationChangedPredicate{},
				util.HTTPScaledObjectReadyConditionPredicate{},
			),
		)).
		// Trigger a reconcile only when the ScaledObject spec,label or annotation changes.
		// Ignore updates to ScaledObject status
		Owns(&kedav1alpha1.ScaledObject{}, builder.WithPredicates(
			predicate.Or(
				predicate.LabelChangedPredicate{},
				predicate.AnnotationChangedPredicate{},
				util.ScaledObjectSpecChangedPredicate{},
			))).
		Complete(r)
}
