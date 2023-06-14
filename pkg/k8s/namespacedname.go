package k8s

import (
	"github.com/kedacore/keda/v2/pkg/scalers/externalscaler"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kedacore/http-add-on/pkg/util"
)

func NamespacedNameFromObject(obj client.Object) *types.NamespacedName {
	if util.IsNil(obj) {
		return nil
	}

	return &types.NamespacedName{
		Namespace: obj.GetNamespace(),
		Name:      obj.GetName(),
	}
}

func NamespacedNameFromScaledObjectRef(sor *externalscaler.ScaledObjectRef) *types.NamespacedName {
	if sor == nil {
		return nil
	}

	return &types.NamespacedName{
		Namespace: sor.GetNamespace(),
		Name:      sor.GetName(),
	}
}
