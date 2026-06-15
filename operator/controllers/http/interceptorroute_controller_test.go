package http

import (
	"testing"

	kedav1alpha1 "github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	httpv1beta1 "github.com/kedacore/http-add-on/operator/apis/http/v1beta1"
	"github.com/kedacore/http-add-on/pkg/k8s"
)

func newInterceptorRouteTestScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	utilruntime.Must(httpv1beta1.AddToScheme(scheme))
	utilruntime.Must(kedav1alpha1.AddToScheme(scheme))
	return scheme
}

func TestInterceptorRouteReconcile_NotFound(t *testing.T) {
	scheme := newInterceptorRouteTestScheme()

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
	scheme := newInterceptorRouteTestScheme()

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
			ScalingMetric: httpv1beta1.ScalingMetricSpec{
				Concurrency: &httpv1beta1.ConcurrencyTargetSpec{
					TargetValue: 5,
				},
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

func TestInterceptorRouteReconcile_SyncsScaledObjectTriggerMetadata(t *testing.T) {
	scheme := newInterceptorRouteTestScheme()

	ir := &httpv1beta1.InterceptorRoute{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "my-route",
		},
		Spec: httpv1beta1.InterceptorRouteSpec{
			Target: httpv1beta1.TargetRef{
				Service: "my-svc",
				Port:    8080,
			},
			ScalingMetric: httpv1beta1.ScalingMetricSpec{
				Concurrency: &httpv1beta1.ConcurrencyTargetSpec{
					TargetValue: 5,
				},
			},
		},
	}

	so := &kedav1alpha1.ScaledObject{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "my-so",
		},
		Spec: kedav1alpha1.ScaledObjectSpec{
			ScaleTargetRef: &kedav1alpha1.ScaleTarget{Name: "my-deploy"},
			Triggers: []kedav1alpha1.ScaleTriggers{
				{
					Type: "external-push",
					Metadata: map[string]string{
						k8s.ScalerAddressKey:    "scaler:9090",
						k8s.InterceptorRouteKey: "my-route",
					},
				},
			},
		},
	}

	cl := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(ir, so).
		WithStatusSubresource(ir).
		Build()

	reconciler := &InterceptorRouteReconciler{
		Client: cl,
		Scheme: cl.Scheme(),
	}

	nn := types.NamespacedName{Namespace: ir.Namespace, Name: ir.Name}
	if _, err := reconciler.Reconcile(t.Context(), ctrl.Request{NamespacedName: nn}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var updatedSO kedav1alpha1.ScaledObject
	if err := cl.Get(t.Context(), types.NamespacedName{Namespace: so.Namespace, Name: so.Name}, &updatedSO); err != nil {
		t.Fatalf("unexpected error getting ScaledObject: %v", err)
	}

	wantHash, err := scalingMetricHash(ir.Spec.ScalingMetric)
	if err != nil {
		t.Fatalf("unexpected error computing hash: %v", err)
	}

	gotHash := updatedSO.Spec.Triggers[0].Metadata[ScalingMetricHashKey]
	if gotHash != wantHash {
		t.Errorf("got scalingMetricHash %q, want %q", gotHash, wantHash)
	}
}

func TestInterceptorRouteReconcile_SkipsUnrelatedScaledObjects(t *testing.T) {
	scheme := newInterceptorRouteTestScheme()

	ir := &httpv1beta1.InterceptorRoute{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "my-route",
		},
		Spec: httpv1beta1.InterceptorRouteSpec{
			Target: httpv1beta1.TargetRef{
				Service: "my-svc",
				Port:    8080,
			},
			ScalingMetric: httpv1beta1.ScalingMetricSpec{
				Concurrency: &httpv1beta1.ConcurrencyTargetSpec{
					TargetValue: 10,
				},
			},
		},
	}

	unrelatedSO := &kedav1alpha1.ScaledObject{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "unrelated-so",
		},
		Spec: kedav1alpha1.ScaledObjectSpec{
			ScaleTargetRef: &kedav1alpha1.ScaleTarget{Name: "other-deploy"},
			Triggers: []kedav1alpha1.ScaleTriggers{
				{
					Type: "external-push",
					Metadata: map[string]string{
						k8s.ScalerAddressKey:    "scaler:9090",
						k8s.InterceptorRouteKey: "other-route",
					},
				},
			},
		},
	}

	cl := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(ir, unrelatedSO).
		WithStatusSubresource(ir).
		Build()

	reconciler := &InterceptorRouteReconciler{
		Client: cl,
		Scheme: cl.Scheme(),
	}

	nn := types.NamespacedName{Namespace: ir.Namespace, Name: ir.Name}
	if _, err := reconciler.Reconcile(t.Context(), ctrl.Request{NamespacedName: nn}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var updatedSO kedav1alpha1.ScaledObject
	if err := cl.Get(t.Context(), types.NamespacedName{Namespace: unrelatedSO.Namespace, Name: unrelatedSO.Name}, &updatedSO); err != nil {
		t.Fatalf("unexpected error getting ScaledObject: %v", err)
	}

	if _, ok := updatedSO.Spec.Triggers[0].Metadata[ScalingMetricHashKey]; ok {
		t.Error("unrelated ScaledObject should not have scalingMetricHash set")
	}
}

func TestInterceptorRouteReconcile_NoUpdateWhenHashUnchanged(t *testing.T) {
	scheme := newInterceptorRouteTestScheme()

	ir := &httpv1beta1.InterceptorRoute{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "my-route",
		},
		Spec: httpv1beta1.InterceptorRouteSpec{
			Target: httpv1beta1.TargetRef{
				Service: "my-svc",
				Port:    8080,
			},
			ScalingMetric: httpv1beta1.ScalingMetricSpec{
				Concurrency: &httpv1beta1.ConcurrencyTargetSpec{
					TargetValue: 5,
				},
			},
		},
	}

	hash, _ := scalingMetricHash(ir.Spec.ScalingMetric)

	so := &kedav1alpha1.ScaledObject{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       "default",
			Name:            "my-so",
			ResourceVersion: "100",
		},
		Spec: kedav1alpha1.ScaledObjectSpec{
			ScaleTargetRef: &kedav1alpha1.ScaleTarget{Name: "my-deploy"},
			Triggers: []kedav1alpha1.ScaleTriggers{
				{
					Type: "external-push",
					Metadata: map[string]string{
						k8s.ScalerAddressKey:    "scaler:9090",
						k8s.InterceptorRouteKey: "my-route",
						ScalingMetricHashKey:    hash,
					},
				},
			},
		},
	}

	cl := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(ir, so).
		WithStatusSubresource(ir).
		Build()

	reconciler := &InterceptorRouteReconciler{
		Client: cl,
		Scheme: cl.Scheme(),
	}

	nn := types.NamespacedName{Namespace: ir.Namespace, Name: ir.Name}
	if _, err := reconciler.Reconcile(t.Context(), ctrl.Request{NamespacedName: nn}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var updatedSO kedav1alpha1.ScaledObject
	if err := cl.Get(t.Context(), types.NamespacedName{Namespace: so.Namespace, Name: so.Name}, &updatedSO); err != nil {
		t.Fatalf("unexpected error getting ScaledObject: %v", err)
	}

	if updatedSO.ResourceVersion != so.ResourceVersion {
		t.Error("ScaledObject should not have been updated when hash is unchanged")
	}
}

func TestUpdateTriggerMetadataHash(t *testing.T) {
	tests := []struct {
		name         string
		triggers     []kedav1alpha1.ScaleTriggers
		irName       string
		hash         string
		wantModified bool
		wantHash     string
	}{
		{
			name: "updates matching trigger",
			triggers: []kedav1alpha1.ScaleTriggers{
				{
					Type: "external-push",
					Metadata: map[string]string{
						k8s.InterceptorRouteKey: "my-route",
					},
				},
			},
			irName:       "my-route",
			hash:         "abc123",
			wantModified: true,
			wantHash:     "abc123",
		},
		{
			name: "skips non-matching trigger",
			triggers: []kedav1alpha1.ScaleTriggers{
				{
					Type: "external-push",
					Metadata: map[string]string{
						k8s.InterceptorRouteKey: "other-route",
					},
				},
			},
			irName:       "my-route",
			hash:         "abc123",
			wantModified: false,
		},
		{
			name: "skips trigger with nil metadata",
			triggers: []kedav1alpha1.ScaleTriggers{
				{Type: "external-push"},
			},
			irName:       "my-route",
			hash:         "abc123",
			wantModified: false,
		},
		{
			name: "no-op when hash already matches",
			triggers: []kedav1alpha1.ScaleTriggers{
				{
					Type: "external-push",
					Metadata: map[string]string{
						k8s.InterceptorRouteKey: "my-route",
						ScalingMetricHashKey:    "abc123",
					},
				},
			},
			irName:       "my-route",
			hash:         "abc123",
			wantModified: false,
		},
		{
			name: "updates only matching trigger among many",
			triggers: []kedav1alpha1.ScaleTriggers{
				{
					Type: "cpu",
					Metadata: map[string]string{
						"type": "Utilization",
					},
				},
				{
					Type: "external-push",
					Metadata: map[string]string{
						k8s.InterceptorRouteKey: "my-route",
					},
				},
			},
			irName:       "my-route",
			hash:         "def456",
			wantModified: true,
			wantHash:     "def456",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			so := &kedav1alpha1.ScaledObject{
				Spec: kedav1alpha1.ScaledObjectSpec{
					Triggers: tt.triggers,
				},
			}

			got := updateTriggerMetadataHash(so, tt.irName, tt.hash)
			if got != tt.wantModified {
				t.Errorf("updateTriggerMetadataHash() = %v, want %v", got, tt.wantModified)
			}

			if tt.wantModified {
				for _, trigger := range so.Spec.Triggers {
					if trigger.Metadata[k8s.InterceptorRouteKey] == tt.irName {
						if trigger.Metadata[ScalingMetricHashKey] != tt.wantHash {
							t.Errorf("got hash %q, want %q", trigger.Metadata[ScalingMetricHashKey], tt.wantHash)
						}
					}
				}
			}
		})
	}
}

func TestScalingMetricHash_Deterministic(t *testing.T) {
	spec := httpv1beta1.ScalingMetricSpec{
		Concurrency: &httpv1beta1.ConcurrencyTargetSpec{
			TargetValue: 5,
		},
	}

	h1, err := scalingMetricHash(spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	h2, err := scalingMetricHash(spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if h1 != h2 {
		t.Errorf("hash should be deterministic: %q != %q", h1, h2)
	}
}

func TestScalingMetricHash_DiffersOnChange(t *testing.T) {
	spec1 := httpv1beta1.ScalingMetricSpec{
		Concurrency: &httpv1beta1.ConcurrencyTargetSpec{
			TargetValue: 5,
		},
	}
	spec2 := httpv1beta1.ScalingMetricSpec{
		Concurrency: &httpv1beta1.ConcurrencyTargetSpec{
			TargetValue: 6,
		},
	}

	h1, _ := scalingMetricHash(spec1)
	h2, _ := scalingMetricHash(spec2)

	if h1 == h2 {
		t.Error("different specs should produce different hashes")
	}
}
