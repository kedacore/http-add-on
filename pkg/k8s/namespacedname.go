package k8s

import (
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
