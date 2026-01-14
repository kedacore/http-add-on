package routing

import (
	iradix "github.com/hashicorp/go-immutable-radix/v2"
	"k8s.io/apimachinery/pkg/types"

	httpv1alpha1 "github.com/kedacore/http-add-on/operator/apis/http/v1alpha1"
	"github.com/kedacore/http-add-on/pkg/k8s"
)

type TableMemory interface {
	Remember(httpso *httpv1alpha1.HTTPScaledObject) TableMemory
	Recall(namespacedName *types.NamespacedName) *httpv1alpha1.HTTPScaledObject
	Forget(namespacedName *types.NamespacedName) TableMemory
	Route(hostname, path string) *httpv1alpha1.HTTPScaledObject
}

type tableMemory struct {
	index *iradix.Tree[*httpv1alpha1.HTTPScaledObject]
	store *iradix.Tree[*httpv1alpha1.HTTPScaledObject]
}

func NewTableMemory() TableMemory {
	return tableMemory{
		index: iradix.New[*httpv1alpha1.HTTPScaledObject](),
		store: iradix.New[*httpv1alpha1.HTTPScaledObject](),
	}
}

var _ TableMemory = (*tableMemory)(nil)

func (tm tableMemory) Remember(httpso *httpv1alpha1.HTTPScaledObject) TableMemory {
	if httpso == nil {
		return tm
	}
	httpso = httpso.DeepCopy()

	indexKey := newTableMemoryIndexKeyFromHTTPSO(httpso)
	index, _, _ := tm.index.Insert(indexKey, httpso)

	keys := NewKeysFromHTTPSO(httpso)
	store := tm.store
	for _, key := range keys {
		newStore, oldHTTPSO, _ := store.Insert(key, httpso)

		// oldest HTTPScaledObject has precedence
		if oldHTTPSO != nil && httpso.GetCreationTimestamp().After(oldHTTPSO.GetCreationTimestamp().Time) {
			continue
		}

		store = newStore
	}

	return tableMemory{
		index: index,
		store: store,
	}
}

func (tm tableMemory) Recall(namespacedName *types.NamespacedName) *httpv1alpha1.HTTPScaledObject {
	if namespacedName == nil {
		return nil
	}

	indexKey := newTableMemoryIndexKey(namespacedName)
	httpso, _ := tm.index.Get(indexKey)
	if httpso == nil {
		return nil
	}

	return httpso.DeepCopy()
}

func (tm tableMemory) Forget(namespacedName *types.NamespacedName) TableMemory {
	if namespacedName == nil {
		return nil
	}

	indexKey := newTableMemoryIndexKey(namespacedName)
	index, httpso, _ := tm.index.Delete(indexKey)
	if httpso == nil {
		return tm
	}

	keys := NewKeysFromHTTPSO(httpso)
	store := tm.store
	for _, key := range keys {
		newStore, oldHTTPSO, _ := store.Delete(key)

		// delete only if namespaced names match
		if oldNamespacedName := k8s.NamespacedNameFromObject(oldHTTPSO); oldNamespacedName == nil || *oldNamespacedName != *namespacedName {
			continue
		}

		store = newStore
	}

	return tableMemory{
		index: index,
		store: store,
	}
}

// Route finds an HTTPScaledObject matching hostname and path.
// Tries exact match first, then wildcards, then catch-all.
func (tm tableMemory) Route(hostname, path string) *httpv1alpha1.HTTPScaledObject {
	// Try exact match
	key := NewKey(hostname, path)
	_, httpso, _ := tm.store.Root().LongestPrefix(key)
	if httpso != nil {
		return httpso
	}

	// Try wildcard matches (most specific to least specific)
	for _, wildcardName := range wildcardHostnames(hostname) {
		wildcardKey := NewKey(wildcardName, path)
		_, httpso, _ = tm.store.Root().LongestPrefix(wildcardKey)
		if httpso != nil {
			return httpso
		}
	}

	// Try catch-all
	catchAllKey := NewKey(catchAllHostKey, path)
	_, httpso, _ = tm.store.Root().LongestPrefix(catchAllKey)
	return httpso
}

type tableMemoryIndexKey []byte

func newTableMemoryIndexKey(namespacedName *types.NamespacedName) tableMemoryIndexKey {
	if namespacedName == nil {
		return nil
	}

	return []byte(namespacedName.String())
}

func newTableMemoryIndexKeyFromHTTPSO(httpso *httpv1alpha1.HTTPScaledObject) tableMemoryIndexKey {
	if httpso == nil {
		return nil
	}

	namespacedName := k8s.NamespacedNameFromObject(httpso)
	return newTableMemoryIndexKey(namespacedName)
}
