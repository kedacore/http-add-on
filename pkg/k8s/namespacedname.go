package k8s

import (
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func NamespacedNameFromObject(obj client.Object) *types.NamespacedName {
	if obj == nil {
		return nil
	}

	return &types.NamespacedName{
		Namespace: obj.GetNamespace(),
		Name:      obj.GetName(),
	}
}
