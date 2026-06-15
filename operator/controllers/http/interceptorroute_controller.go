package http

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"

	kedav1alpha1 "github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	httpv1beta1 "github.com/kedacore/http-add-on/operator/apis/http/v1beta1"
	"github.com/kedacore/http-add-on/pkg/k8s"
)

const (
	// ScalingMetricHashKey is the trigger metadata key used to store a hash of
	// the InterceptorRoute's scaling metric spec. Updating this value forces
	// KEDA to re-reconcile the ScaledObject and refresh the HPA targets.
	ScalingMetricHashKey = "scalingMetricHash"
)

// InterceptorRouteReconciler reconciles InterceptorRoute objects.
type InterceptorRouteReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=http.keda.sh,resources=interceptorroutes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=http.keda.sh,resources=interceptorroutes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=http.keda.sh,resources=interceptorroutes/finalizers,verbs=update
// +kubebuilder:rbac:groups=keda.sh,resources=scaledobjects,verbs=get;list;watch;update;patch

func (r *InterceptorRouteReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var ir httpv1beta1.InterceptorRoute
	if err := r.Get(ctx, req.NamespacedName, &ir); err != nil {
		if k8serrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		logger.Error(err, "Failed to get InterceptorRoute")
		return ctrl.Result{}, err
	}

	if err := r.syncScaledObjects(ctx, &ir); err != nil {
		logger.Error(err, "Failed to sync ScaledObjects")

		meta.SetStatusCondition(&ir.Status.Conditions, metav1.Condition{
			Type:               httpv1beta1.ConditionTypeReady,
			Status:             metav1.ConditionFalse,
			ObservedGeneration: ir.Generation,
			Reason:             httpv1beta1.ConditionReasonScaledObjectSyncError,
			Message:            fmt.Sprintf("Failed to sync ScaledObjects: %v", err),
		})
		if statusErr := r.Client.Status().Update(ctx, &ir); statusErr != nil {
			logger.Error(statusErr, "Failed to update status")
		}
		return ctrl.Result{}, err
	}

	meta.SetStatusCondition(&ir.Status.Conditions, metav1.Condition{
		Type:               httpv1beta1.ConditionTypeReady,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: ir.Generation,
		Reason:             httpv1beta1.ConditionReasonReconciled,
		Message:            "InterceptorRoute reconciled",
	})
	if err := r.Client.Status().Update(ctx, &ir); err != nil {
		logger.Error(err, "Failed to update status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// syncScaledObjects finds ScaledObjects whose triggers reference this
// InterceptorRoute and patches their trigger metadata with a hash of
// the current scaling metric spec. The spec change bumps the ScaledObject
// generation, which causes KEDA to rebuild its scaler cache, re-invoke
// GetMetricSpec on the external scaler, and update the HPA targets.
func (r *InterceptorRouteReconciler) syncScaledObjects(ctx context.Context, ir *httpv1beta1.InterceptorRoute) error {
	logger := log.FromContext(ctx)

	hash, err := scalingMetricHash(ir.Spec.ScalingMetric)
	if err != nil {
		return fmt.Errorf("computing scaling metric hash: %w", err)
	}

	var scaledObjects kedav1alpha1.ScaledObjectList
	if err := r.List(ctx, &scaledObjects, client.InNamespace(ir.Namespace)); err != nil {
		return fmt.Errorf("listing ScaledObjects: %w", err)
	}

	var errs []error
	for i := range scaledObjects.Items {
		so := &scaledObjects.Items[i]
		base := so.DeepCopy()
		if !updateTriggerMetadataHash(so, ir.Name, hash) {
			continue
		}

		logger.Info("Updating ScaledObject trigger metadata",
			"scaledObject", so.Name,
			"scalingMetricHash", hash,
		)
		if err := r.Patch(ctx, so, client.MergeFrom(base)); err != nil {
			if k8serrors.IsNotFound(err) {
				logger.V(1).Info("ScaledObject deleted before patch, skipping",
					"scaledObject", so.Name)
				continue
			}
			errs = append(errs, fmt.Errorf("patching ScaledObject %s: %w", so.Name, err))
		}
	}

	return errors.Join(errs...)
}

// updateTriggerMetadataHash sets ScalingMetricHashKey in every trigger that
// references the given InterceptorRoute. Returns true when at least one
// trigger was modified.
func updateTriggerMetadataHash(so *kedav1alpha1.ScaledObject, irName, hash string) bool {
	modified := false
	for j := range so.Spec.Triggers {
		trigger := &so.Spec.Triggers[j]
		if trigger.Metadata == nil {
			continue
		}
		if trigger.Metadata[k8s.InterceptorRouteKey] != irName {
			continue
		}
		if trigger.Metadata[ScalingMetricHashKey] == hash {
			continue
		}
		trigger.Metadata[ScalingMetricHashKey] = hash
		modified = true
	}
	return modified
}

// scalingMetricHash returns a hex-encoded SHA-256 digest of the JSON
// representation of the given ScalingMetricSpec.
func scalingMetricHash(spec httpv1beta1.ScalingMetricSpec) (string, error) {
	data, err := json.Marshal(spec)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", sha256.Sum256(data)), nil
}

func (r *InterceptorRouteReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&httpv1beta1.InterceptorRoute{}, builder.WithPredicates(
			predicate.GenerationChangedPredicate{},
		)).
		Named("interceptorroute").
		Complete(r)
}
