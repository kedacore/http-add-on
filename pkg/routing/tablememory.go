package routing

import (
	iradix "github.com/hashicorp/go-immutable-radix/v2"

	httpv1alpha1 "github.com/kedacore/http-add-on/operator/apis/http/v1alpha1"
)

// TableMemory is an immutable routing table for HTTPScaledObjects.
type TableMemory struct {
	store *iradix.Tree[*httpv1alpha1.HTTPScaledObject]
}

// NewTableMemory creates an empty TableMemory.
func NewTableMemory() *TableMemory {
	return &TableMemory{
		store: iradix.New[*httpv1alpha1.HTTPScaledObject](),
	}
}

// Remember adds an HTTPScaledObject and returns a new TableMemory.
func (tm *TableMemory) Remember(httpso *httpv1alpha1.HTTPScaledObject) *TableMemory {
	if httpso == nil {
		return tm
	}
	httpso = httpso.DeepCopy()

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

	return &TableMemory{
		store: store,
	}
}

// Route finds an HTTPScaledObject matching hostname and path.
// Tries exact match first, then wildcards, then catch-all.
func (tm *TableMemory) Route(hostname, path string) *httpv1alpha1.HTTPScaledObject {
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
