package routing

import (
	iradix "github.com/hashicorp/go-immutable-radix/v2"
	httpv1alpha1 "github.com/kedacore/http-add-on/operator/apis/http/v1alpha1"
)

type httpSOIndex struct {
	radix *iradix.Tree[*httpv1alpha1.HTTPScaledObject]
}

func newHTTPSOIndex() *httpSOIndex {
	return &httpSOIndex{radix: iradix.New[*httpv1alpha1.HTTPScaledObject]()}
}

func (hi *httpSOIndex) insert(key tableMemoryIndexKey, httpso *httpv1alpha1.HTTPScaledObject) (*httpSOIndex, *httpv1alpha1.HTTPScaledObject, bool) {
	newRadix, oldVal, oldSet := hi.radix.Insert(key, httpso)
	newHttpSOIndex := &httpSOIndex{
		radix: newRadix,
	}
	return newHttpSOIndex, oldVal, oldSet
}

func (hi *httpSOIndex) get(key tableMemoryIndexKey) (*httpv1alpha1.HTTPScaledObject, bool) {
	return hi.radix.Get(key)
}

func (hi *httpSOIndex) delete(key tableMemoryIndexKey) (*httpSOIndex, *httpv1alpha1.HTTPScaledObject, bool) {
	newRadix, oldVal, oldSet := hi.radix.Delete(key)
	newHttpSOIndex := &httpSOIndex{
		radix: newRadix,
	}
	return newHttpSOIndex, oldVal, oldSet
}
