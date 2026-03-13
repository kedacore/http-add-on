package http

import (
	"testing"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	httpv1beta1 "github.com/kedacore/http-add-on/operator/apis/http/v1beta1"
)

func TestInterceptorRouteReconcile_NotFound(t *testing.T) {
	scheme := runtime.NewScheme()
	utilruntime.Must(httpv1beta1.AddToScheme(scheme))

	client := fake.NewClientBuilder().WithScheme(scheme).Build()

	reconciler := &InterceptorRouteReconciler{
		Client: client,
		Scheme: client.Scheme(),
	}

	result, err := reconciler.Reconcile(t.Context(), ctrl.Request{
		NamespacedName: types.NamespacedName{
			Namespace: "default",
			Name:      "does-not-exist",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != (ctrl.Result{}) {
		t.Fatalf("expected empty result, got %v", result)
	}
}

func TestInterceptorRouteReconcile_SetsReadyCondition(t *testing.T) {
	scheme := runtime.NewScheme()
	utilruntime.Must(httpv1beta1.AddToScheme(scheme))

	ir := &httpv1beta1.InterceptorRoute{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "test-route",
		},
		Spec: httpv1beta1.InterceptorRouteSpec{
			Target: httpv1beta1.TargetRef{
				Service: "test-service",
				Port:    8080,
			},
		},
	}

	client := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(ir).
		WithStatusSubresource(ir).
		Build()

	reconciler := &InterceptorRouteReconciler{
		Client: client,
		Scheme: client.Scheme(),
	}

	nn := types.NamespacedName{
		Namespace: ir.Namespace,
		Name:      ir.Name,
	}
	_, err := reconciler.Reconcile(t.Context(), ctrl.Request{
		NamespacedName: nn,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var updated httpv1beta1.InterceptorRoute
	if err := client.Get(t.Context(), nn, &updated); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cond := meta.FindStatusCondition(updated.Status.Conditions, httpv1beta1.ConditionTypeReady)
	if cond == nil {
		t.Fatal("expected Ready condition was not found")
	}

	if cond.Status != metav1.ConditionTrue {
		t.Errorf("got Ready status %s, want %s", cond.Status, metav1.ConditionTrue)
	}
	if cond.Reason != httpv1beta1.ConditionReasonReconciled {
		t.Errorf("got Ready reason %q, want %q", cond.Reason, httpv1beta1.ConditionReasonReconciled)
	}
}
