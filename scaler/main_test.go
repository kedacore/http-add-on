package main

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	toolscache "k8s.io/client-go/tools/cache"

	httpv1beta1 "github.com/kedacore/http-add-on/operator/apis/http/v1beta1"
)

func TestScalingMetricChanged(t *testing.T) {
	newObj := func(target int32) *httpv1beta1.InterceptorRoute {
		return &httpv1beta1.InterceptorRoute{
			Spec: httpv1beta1.InterceptorRouteSpec{
				ScalingMetric: httpv1beta1.ScalingMetricSpec{
					Concurrency: &httpv1beta1.ConcurrencyTargetSpec{TargetValue: target},
				},
			},
		}
	}

	tests := map[string]struct {
		oldObj any
		newObj any
		want   bool
	}{
		"same scaling metric": {
			oldObj: newObj(5),
			newObj: newObj(5),
			want:   false,
		},
		"changed scaling metric": {
			oldObj: newObj(5),
			newObj: newObj(10),
			want:   true,
		},
		"unknown old object": {
			oldObj: toolscache.DeletedFinalStateUnknown{},
			newObj: newObj(5),
			want:   true,
		},
		"unknown new object": {
			oldObj: newObj(5),
			newObj: toolscache.DeletedFinalStateUnknown{},
			want:   true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if got := scalingMetricChanged(tc.oldObj, tc.newObj); got != tc.want {
				t.Errorf("scalingMetricChanged() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestObjectNamespacedName(t *testing.T) {
	route := &httpv1beta1.InterceptorRoute{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "test-ns"},
	}
	want := types.NamespacedName{Name: route.Name, Namespace: route.Namespace}

	tests := map[string]struct {
		obj     any
		wantErr bool
	}{
		"object": {
			obj: route,
		},
		"delete tombstone": {
			obj: toolscache.DeletedFinalStateUnknown{Key: "test-ns/test", Obj: route},
		},
		"unknown object": {
			obj:     struct{}{},
			wantErr: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := objectNamespacedName(tc.obj)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != want {
				t.Errorf("objectNamespacedName() = %v, want %v", got, want)
			}
		})
	}
}
