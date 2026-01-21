package routing

import (
	"net/http"
	"slices"
	"strings"

	iradix "github.com/hashicorp/go-immutable-radix/v2"

	httpv1alpha1 "github.com/kedacore/http-add-on/operator/apis/http/v1alpha1"
)

// TableMemory is an immutable routing table for HTTPScaledObjects.
type TableMemory struct {
	store *iradix.Tree[[]*httpv1alpha1.HTTPScaledObject]
}

// NewTableMemory creates an empty TableMemory.
func NewTableMemory() *TableMemory {
	return &TableMemory{
		store: iradix.New[[]*httpv1alpha1.HTTPScaledObject](),
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
		store = remember(store, key, httpso)
	}

	return &TableMemory{
		store: store,
	}
}

// Route finds an HTTPScaledObject matching hostname, path and headers.
// Tries exact match first the hostname, then filter by headers and find the most specific match
// then it moves to wildcards with similar header matching rule, finally catch-all.
func (tm *TableMemory) Route(hostname, path string, headers http.Header) *httpv1alpha1.HTTPScaledObject {
	// Try exact match
	key := NewKey(hostname, path)
	_, httpsoList, _ := tm.store.Root().LongestPrefix(key)

	// Filter header matches, the httpsoList is already sorted by specificity so the first match is the most specific one
	for _, httpso := range httpsoList {
		if headersMatch(httpso, headers) {
			return httpso
		}
	}

	// Try wildcard matches (most specific to least specific)
	for _, wildcardName := range wildcardHostnames(hostname) {
		wildcardKey := NewKey(wildcardName, path)
		if httpso := tm.tryWithKey(wildcardKey, headers); httpso != nil {
			return httpso
		}
	}

	// Try catch-all
	catchAllKey := NewKey(catchAllHostKey, path)
	return tm.tryWithKey(catchAllKey, headers)
}

// tryWithKey attempts to find an HTTPScaledObject for the given key and headers.
func (tm *TableMemory) tryWithKey(key Key, headers http.Header) *httpv1alpha1.HTTPScaledObject {
	_, httpsoList, _ := tm.store.Root().LongestPrefix(key)
	for _, httpso := range httpsoList {
		if headersMatch(httpso, headers) {
			return httpso
		}
	}
	return nil
}

// headersMatch checks if the provided headers match the HTTPScaledObject's header requirements.
func headersMatch(httpso *httpv1alpha1.HTTPScaledObject, reqHeaders http.Header) bool {
	if httpso == nil || len(httpso.Spec.Headers) == 0 {
		// No header requirements, always match
		return true
	}

	for _, hsoHeader := range httpso.Spec.Headers {
		reqHeaderValues := reqHeaders.Values(hsoHeader.Name)
		if hsoHeader.Value == nil && len(reqHeaderValues) == 0 {
			// Required header not present
			return false
		}

		if hsoHeader.Value != nil && !slices.Contains(reqHeaderValues, *hsoHeader.Value) {
			// Required header with value not present
			return false
		}
	}

	return true
}

// remember adds an HTTPScaledObject to the store for the given key, sorting by specificity.
func remember(store *iradix.Tree[[]*httpv1alpha1.HTTPScaledObject], key Key, httpso *httpv1alpha1.HTTPScaledObject) *iradix.Tree[[]*httpv1alpha1.HTTPScaledObject] {
	existing, found := store.Root().Get(key)
	if !found {
		// No existing entry, create a new one
		newStore, _, _ := store.Insert(key, []*httpv1alpha1.HTTPScaledObject{httpso})
		return newStore
	}
	existing = slices.Clone(existing)

	if i := slices.IndexFunc(existing, func(hi *httpv1alpha1.HTTPScaledObject) bool {
		return hi.Name == httpso.Name && hi.Namespace == httpso.Namespace
	}); i != -1 { // Update existing entry
		existing[i] = httpso
	} else { // Add new entry
		existing = append(existing, httpso)
	}

	slices.SortFunc(existing, func(hi, hj *httpv1alpha1.HTTPScaledObject) int {
		// first by number of headers, more headers first
		hih, hjh := len(hi.Spec.Headers), len(hj.Spec.Headers)
		if hih != hjh {
			return hjh - hih
		}
		// then by specificity of header values, more specific first
		specificityI, specificityJ := specificityOfHeaders(hi), specificityOfHeaders(hj)
		if specificityI != specificityJ {
			return specificityJ - specificityI
		}

		// then by creation timestamp, older first
		if diff := hi.CreationTimestamp.Compare(hj.CreationTimestamp.Time); diff != 0 {
			return diff
		}
		// tiebreaker by namespace/name, lexicographically (descending)
		if diff := strings.Compare(hi.Namespace, hj.Namespace); diff != 0 {
			return -diff
		}
		return -strings.Compare(hi.Name, hj.Name)
	})

	newRadix, _, _ := store.Insert(key, existing)
	return newRadix
}

// specificityOfHeaders calculates how specific the headers are in the HTTPScaledObject putting more weight on headers with values.
func specificityOfHeaders(httpso *httpv1alpha1.HTTPScaledObject) int {
	if httpso == nil || httpso.Spec.Headers == nil {
		return 0
	}
	specificity := 0
	for _, header := range httpso.Spec.Headers {
		if header.Value != nil {
			specificity++
		}
	}
	return specificity
}
