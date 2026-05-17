package util

import (
	kedav1alpha1 "github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	"k8s.io/apimachinery/pkg/api/equality"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

type AnnotationKeyChangedPredicate struct {
	predicate.Funcs
	Keys []string
}

func (p AnnotationKeyChangedPredicate) Update(e event.UpdateEvent) bool {
	if e.ObjectOld == nil || e.ObjectNew == nil {
		return false
	}

	oldAnnotations := e.ObjectOld.GetAnnotations()
	newAnnotations := e.ObjectNew.GetAnnotations()

	for _, key := range p.Keys {
		if oldAnnotations[key] != newAnnotations[key] {
			return true
		}
	}
	return false
}


type ScaledObjectSpecChangedPredicate struct {
	predicate.Funcs
}

func (ScaledObjectSpecChangedPredicate) Update(e event.UpdateEvent) bool {
	newObj := e.ObjectNew.(*kedav1alpha1.ScaledObject)
	oldObj := e.ObjectOld.(*kedav1alpha1.ScaledObject)

	return !equality.Semantic.DeepDerivative(newObj.Spec, oldObj.Spec)
}
