package k8s

import (
	"fmt"

	"k8s.io/apimachinery/pkg/types"
)

func ResourceKeyFromNamespacedName(nn types.NamespacedName) string {
	return ResourceKey(nn.Namespace, nn.Name)
}

func ResourceKey(namespace, name string) string {
	return fmt.Sprintf("%s/%s", namespace, name)
}
