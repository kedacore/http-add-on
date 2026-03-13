package http

import (
	"context"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	httpv1beta1 "github.com/kedacore/http-add-on/operator/apis/http/v1beta1"
)

// InterceptorRouteReconciler reconciles InterceptorRoute objects.
type InterceptorRouteReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=http.keda.sh,resources=interceptorroutes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=http.keda.sh,resources=interceptorroutes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=http.keda.sh,resources=interceptorroutes/finalizers,verbs=update

func (r *InterceptorRouteReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var ir httpv1beta1.InterceptorRoute
	if err := r.Get(ctx, req.NamespacedName, &ir); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		logger.Error(err, "Failed to get InterceptorRoute")
		return ctrl.Result{}, err
	}

	// We only set the Ready condition right now keeping this reconciler basically as a placeholder.
	// TODO(v1): decide if we want to keep and extend or remove this reconciler
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

func (r *InterceptorRouteReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&httpv1beta1.InterceptorRoute{}, builder.WithPredicates(
			predicate.GenerationChangedPredicate{},
		)).
		Named("interceptorroute").
		Complete(r)
}
