package routing

import (
	"net/textproto"
	"sort"

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
func (tm *TableMemory) Route(hostname, path string, headers map[string][]string) *httpv1alpha1.HTTPScaledObject {
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
		_, httpsoList, _ = tm.store.Root().LongestPrefix(wildcardKey)
		for _, httpso := range httpsoList {
			if headersMatch(httpso, headers) {
				return httpso
			}
		}
	}

	// Try catch-all
	catchAllKey := NewKey(catchAllHostKey, path)
	_, httpsoList, _ = tm.store.Root().LongestPrefix(catchAllKey)
	for _, httpso := range httpsoList {
		if headersMatch(httpso, headers) {
			return httpso
		}
	}

	// No match found
	return nil
}

// headersMatch checks if the provided headers match the HTTPScaledObject's header requirements.
func headersMatch(httpso *httpv1alpha1.HTTPScaledObject, reqHeaders map[string][]string) bool {
	if httpso == nil || len(httpso.Spec.Headers) == 0 {
		// No header requirements, always match
		return true
	}

	for _, hsoHeader := range httpso.Spec.Headers {
		hsoHeaderName := textproto.CanonicalMIMEHeaderKey(hsoHeader.Name)
		reqHeaderValues, exists := reqHeaders[hsoHeaderName]
		if !exists {
			return false
		}
		foundMatchingHeaderValue := false
		if hsoHeader.Value == "" {
			// Header presence is enough if no value is specified
			foundMatchingHeaderValue = true
		} else {
			// Check for matching header value if specified
			for _, v := range reqHeaderValues {
				if hsoHeader.Value == v {
					foundMatchingHeaderValue = true
					break
				}
			}
		}
		if !foundMatchingHeaderValue {
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
	newSlice := make([]*httpv1alpha1.HTTPScaledObject, len(existing))
	copy(newSlice, existing)
	existing = newSlice

	foundHSO := false
	for i, existingHTTPSO := range existing {
		if existingHTTPSO.Name == httpso.Name && existingHTTPSO.Namespace == httpso.Namespace {
			// Update existing entry
			foundHSO = true
			existing[i] = httpso
			break
		}
	}
	if !foundHSO {
		existing = append(existing, httpso)
	}

	sort.Slice(existing, func(i, j int) bool {
		// first by number of headers, more headers first
		hi, hj := existing[i], existing[j]
		hih, hjh := len(hi.Spec.Headers), len(hj.Spec.Headers)
		if hih != hjh {
			return hih > hjh
		}
		// then by specificity of header values, more specific first
		specificityI, specificityJ := specificityOfHeaders(hi), specificityOfHeaders(hj)
		if specificityI != specificityJ {
			return specificityI > specificityJ
		}

		// then by creation timestamp, older first
		if !hi.CreationTimestamp.Time.Equal(hj.CreationTimestamp.Time) {
			return hj.CreationTimestamp.After(hi.CreationTimestamp.Time)
		}
		// tiebreaker by namespace/name, lexicographically (descending)
		if hi.Namespace != hj.Namespace {
			return hi.Namespace > hj.Namespace
		}
		return hi.Name > hj.Name
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
		if header.Value != "" {
			specificity++
		}
	}
	return specificity
}
