package cache

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/runtime"

	httpv1alpha1 "github.com/kedacore/http-add-on/operator/apis/http/v1alpha1"
)

func TestNewScheme(t *testing.T) {
	tests := map[string]struct {
		kind string
		obj  runtime.Object
	}{
		"corev1": {
			kind: "Service",
			obj:  &corev1.Service{},
		},
		"discoveryv1": {
			kind: "EndpointSlice",
			obj:  &discoveryv1.EndpointSlice{},
		},
		"httpv1alpha1": {
			kind: "HTTPScaledObject",
			obj:  &httpv1alpha1.HTTPScaledObject{},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			scheme := NewScheme()
			if scheme == nil {
				t.Fatal("expected non-nil schema")
			}

			gvks, _, err := scheme.ObjectKinds(tt.obj)
			if err != nil {
				t.Fatalf("expected kind %q to be registered", tt.kind)
			}

			if len(gvks) == 0 {
				t.Fatalf("expected at least one GVK for kind %q", tt.kind)
			}
		})
	}
}
