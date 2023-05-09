package routing

import (
	"context"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func toNamespacedName(obj client.Object) types.NamespacedName {
	return types.NamespacedName{
		Namespace: obj.GetNamespace(),
		Name:      obj.GetName(),
	}
}

//revive:disable:context-as-argument
func applyContext(f func(ctx context.Context) error, ctx context.Context) func() error {
	//revive:enable:context-as-argument
	return func() error {
		return f(ctx)
	}
}
