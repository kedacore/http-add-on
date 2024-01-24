package util

import (
	kedav1alpha1 "github.com/kedacore/keda/v2/apis/keda/v1alpha1"

	"github.com/kedacore/http-add-on/operator/apis/http/v1alpha1"
	"k8s.io/apimachinery/pkg/api/equality"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

type HTTPScaledObjectReadyConditionPredicate struct {
	predicate.Funcs
}

func (HTTPScaledObjectReadyConditionPredicate) Update(e event.UpdateEvent) bool {
	if e.ObjectOld == nil || e.ObjectNew == nil {
		return false
	}

	var newReadyCondition, oldReadyCondition v1alpha1.HTTPScaledObjectCondition

	oldObj, ok := e.ObjectOld.(*v1alpha1.HTTPScaledObject)
	if !ok {
		return false
	}
	oldReadyCondition = oldObj.Status.Conditions.GetReadyCondition()

	newObj, ok := e.ObjectNew.(*v1alpha1.HTTPScaledObject)
	if !ok {
		return false
	}
	newReadyCondition = newObj.Status.Conditions.GetReadyCondition()

	// False/Unknown -> True
	if !oldReadyCondition.IsTrue() && newReadyCondition.IsTrue() {
		return true
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
