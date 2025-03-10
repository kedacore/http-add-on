package routing

import (
	"net/textproto"

	"k8s.io/apimachinery/pkg/types"

	httpv1alpha1 "github.com/kedacore/http-add-on/operator/apis/http/v1alpha1"
	"github.com/kedacore/http-add-on/pkg/k8s"
)

type TableMemory interface {
	Remember(httpso *httpv1alpha1.HTTPScaledObject) TableMemory
	Recall(namespacedName *types.NamespacedName) *httpv1alpha1.HTTPScaledObject
	Forget(namespacedName *types.NamespacedName) TableMemory
	Route(key Key) *httpv1alpha1.HTTPScaledObject
	RouteWithHeaders(key Key, httpHeaders map[string][]string) *httpv1alpha1.HTTPScaledObject
}

type tableMemory struct {
	index *httpSOIndex
	store *httpSOStore
}

func newTableMemory() tableMemory {
	return tableMemory{
		index: newHTTPSOIndex(),
		store: newHTTPSOStore(),
	}
}

func NewTableMemory() TableMemory {
	return tableMemory{
		index: newHTTPSOIndex(),
		store: newHTTPSOStore(),
	}
}

var _ TableMemory = (*tableMemory)(nil)

func (tm tableMemory) Remember(httpso *httpv1alpha1.HTTPScaledObject) TableMemory {
	if httpso == nil {
		return tm
	}
	httpso = httpso.DeepCopy()

	indexKey := newTableMemoryIndexKeyFromHTTPSO(httpso)
	index, _, _ := tm.index.insert(indexKey, httpso)

	keys := NewKeysFromHTTPSO(httpso)
	store := tm.store
	for _, key := range keys {
		store = store.append(key, httpso)
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
	httpso, _ := tm.index.get(indexKey)
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
	index, httpso, oldSet := tm.index.delete(indexKey)
	if httpso == nil || oldSet == false {
		return tm
	}
	store := tm.store.DeleteAllInstancesOfHTTPSO(httpso)
	return tableMemory{
		index: index,
		store: store,
	}
}

func (tm tableMemory) Route(key Key) *httpv1alpha1.HTTPScaledObject {
	_, httpsoList, _ := tm.store.GetLongestPrefix(key)
	if httpsoList == nil || len(httpsoList.Items) == 0 {
		return nil
	}
	return httpsoList.Items[0]
}

func (tm tableMemory) RouteWithHeaders(key Key, httpHeaders map[string][]string) *httpv1alpha1.HTTPScaledObject {
	_, httpsoList, _ := tm.store.GetLongestPrefix(key)
	if httpsoList == nil || len(httpsoList.Items) == 0 {
		return nil
	}
	if httpHeaders == nil || len(httpHeaders) == 0 {
		return httpsoList.Items[0]
	}
	var httpsoWithoutHeaders *httpv1alpha1.HTTPScaledObject

	// route to first httpso which has a matching header
	for _, httpso := range httpsoList.Items {
		if httpso.Spec.Headers != nil {
			for _, header := range httpso.Spec.Headers {
				// normalize header spacing how golang does it
				canonicalHeaderName := textproto.CanonicalMIMEHeaderKey(header.Name)
				if headerValues, exists := httpHeaders[canonicalHeaderName]; exists {
					for _, v := range headerValues {
						if header.Value == v {
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
