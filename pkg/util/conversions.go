package util

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kedacore/http-add-on/operator/apis/http/v1alpha1"
)

func GetHTTPScalingSetSpecFromObject(httpss metav1.Object) *v1alpha1.HTTPScalingSetSpec {
	var httpSpec *v1alpha1.HTTPScalingSetSpec
	switch obj := httpss.(type) {
	case *v1alpha1.ClusterHTTPScalingSet:
		httpSpec = &obj.Spec
	case *v1alpha1.HTTPScalingSet:
		httpSpec = &obj.Spec
	}
	return httpSpec
}

func GetHTTPScalingSetKindFromObject(httpss metav1.Object) v1alpha1.ScalingSetKind {
	var kind v1alpha1.ScalingSetKind
	switch httpss.(type) {
	case *v1alpha1.ClusterHTTPScalingSet:
		kind = v1alpha1.ClusterHTTPScalingSetKind
	case *v1alpha1.HTTPScalingSet:
		kind = v1alpha1.HTTPScalingSetKind
	}
	return kind
}
