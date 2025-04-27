package routing

import (
	iradix "github.com/hashicorp/go-immutable-radix/v2"
	httpv1alpha1 "github.com/kedacore/http-add-on/operator/apis/http/v1alpha1"
	"github.com/kedacore/http-add-on/pkg/k8s"
)

// light wrapper around radix tree containing HTTPScaledObjectList
// with convenience functions to manage CRUD for individual HTTPScaledObject.
// created as an abstraction to manage complexity for tablememory implementation
// the store is meant to map host + path keys to one or more HTTPScaledObject
// and return one arbitrarily or route based on headers
type httpSOStore struct {
	radix *iradix.Tree[*httpv1alpha1.HTTPScaledObjectList]
}

func newHTTPSOStore() *httpSOStore {
	return &httpSOStore{radix: iradix.New[*httpv1alpha1.HTTPScaledObjectList]()}
}

// Insert key value into httpSOStore
// Gets old list of HTTPScaledObjectList
// if exists appends to list and returns new httpSOStore
// with new radix tree
func (hs *httpSOStore) append(key Key, httpso *httpv1alpha1.HTTPScaledObject) *httpSOStore {
	httpsoList, found := hs.radix.Get(key)
	var newHttpSOStore *httpSOStore
	if !found {
		newList := &httpv1alpha1.HTTPScaledObjectList{Items: []*httpv1alpha1.HTTPScaledObject{httpso}}
		newRadix, _, _ := hs.radix.Insert(key, newList)
		newHttpSOStore = &httpSOStore{
			radix: newRadix,
		}
	} else {
		newList := &httpv1alpha1.HTTPScaledObjectList{Items: append(httpsoList.Items, httpso)}
		newRadix, _, _ := hs.radix.Insert(key, newList)
		newHttpSOStore = &httpSOStore{
			radix: newRadix,
		}
	}
	return newHttpSOStore
}

func (hs *httpSOStore) insert(key Key, httpsoList *httpv1alpha1.HTTPScaledObjectList) (*httpSOStore, *httpv1alpha1.HTTPScaledObjectList, bool) {
	newRadix, oldVal, ok := hs.radix.Insert(key, httpsoList)
	newHttpSOStore := &httpSOStore{
		radix: newRadix,
	}
	return newHttpSOStore, oldVal, ok
}

func (hs *httpSOStore) get(key Key) (*httpv1alpha1.HTTPScaledObjectList, bool) {
	return hs.radix.Get(key)
}

func (hs *httpSOStore) delete(key Key) (*httpSOStore, *httpv1alpha1.HTTPScaledObjectList, bool) {
	newRadix, oldVal, oldSet := hs.radix.Delete(key)
	newHttpSOStore := &httpSOStore{
		radix: newRadix,
	}
	return newHttpSOStore, oldVal, oldSet
}

// convenience function
// retrieves all keys associated with HTTPScaledObject
// and deletes it from every list in the store
func (hs *httpSOStore) DeleteAllInstancesOfHTTPSO(httpso *httpv1alpha1.HTTPScaledObject) *httpSOStore {
	httpsoNamespacedName := k8s.NamespacedNameFromObject(httpso)
	newHttpSOStore := &httpSOStore{radix: hs.radix}
	keys := NewKeysFromHTTPSO(httpso)
	for _, key := range keys {
		httpsoList, _ := newHttpSOStore.radix.Get(key)
		for i, httpso := range httpsoList.Items {
			// delete only if namespaced names match
			if currHttpsoNamespacedName := k8s.NamespacedNameFromObject(httpso); *httpsoNamespacedName == *currHttpsoNamespacedName {
				httpsoList.Items = append(httpsoList.Items[:i], httpsoList.Items[i+1:]...)
				break
			}
		}
		if len(httpsoList.Items) == 0 {
			newHttpSOStore.radix, _, _ = newHttpSOStore.radix.Delete(key)
		} else {
			newHttpSOStore.radix, _, _ = newHttpSOStore.radix.Insert(key, httpsoList)
		}
	}
	return newHttpSOStore
}

func (hs *httpSOStore) GetLongestPrefix(key Key) ([]byte, *httpv1alpha1.HTTPScaledObjectList, bool) {
	return hs.radix.Root().LongestPrefix(key)
}
