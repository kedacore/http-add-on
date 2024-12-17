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
	Route(key Key, httpHeaders map[string][]string) *httpv1alpha1.HTTPScaledObject
}

type tableMemory struct {
	index *iradix.Tree[*httpv1alpha1.HTTPScaledObject]
	store *iradix.Tree[[]*httpv1alpha1.HTTPScaledObject]
}

func NewTableMemory() TableMemory {
	return tableMemory{
		index: iradix.New[*httpv1alpha1.HTTPScaledObject](),
		store: iradix.New[[]*httpv1alpha1.HTTPScaledObject](),
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
		httpsoList, found := store.Get(key)
		if !found {
			newStore, _, _ := store.Insert(key, []*httpv1alpha1.HTTPScaledObject{httpso})
			store = newStore
		} else {
			newStore, _, _ := store.Insert(key, append(httpsoList, httpso))
			store = newStore
		}
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
		httpsoList, _ := store.Get(key)
		for i, httpso := range httpsoList {
			// delete only if namespaced names match
			if httpsoNamespacedName := k8s.NamespacedNameFromObject(httpso); *httpsoNamespacedName == *namespacedName {
				httpsoList = append(httpsoList[:i], httpsoList[i+1:]...)
				break
			}
		}
		if len(httpsoList) == 0 {
			newStore, _, _ := store.Delete(key)
			store = newStore
		} else {
			newStore, _, _ := store.Insert(key, httpsoList)
			store = newStore
		}
	}

	return tableMemory{
		index: index,
		store: store,
	}
}

func (tm tableMemory) Route(key Key, httpHeaders map[string][]string) *httpv1alpha1.HTTPScaledObject {
	_, httpsoList, _ := tm.store.Root().LongestPrefix(key)
	if httpsoList == nil || len(httpsoList) == 0 {
		return nil
	}
	if httpHeaders == nil || len(httpHeaders) == 0 {
		return httpsoList[0]
	}
	var httpsoWithoutHeaders *httpv1alpha1.HTTPScaledObject
	// route to first httpso which has a matching header
	for _, httpso := range httpsoList {
		if httpso.Spec.Headers != nil {
			for k, v1 := range httpso.Spec.Headers {
				if headerValues, exists := httpHeaders[k]; exists {
					for _, v2 := range headerValues {
						if v1 == v2 {
							return httpso
						}
					}
				}
			}
		} else if httpsoWithoutHeaders == nil {
			httpsoWithoutHeaders = httpso
		}
	}

	// if no matches via header, route to httpso without headers supplied
	if httpsoWithoutHeaders != nil {
		return httpsoWithoutHeaders
	}

	// otherwise routing fails
	return nil
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
