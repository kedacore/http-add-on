package routing

import (
	"cmp"
	"net/http"
	"slices"

	iradix "github.com/hashicorp/go-immutable-radix/v2"

	httpv1beta1 "github.com/kedacore/http-add-on/operator/apis/http/v1beta1"
)

type routeEntry struct {
	ir      *httpv1beta1.InterceptorRoute
	headers []httpv1beta1.HeaderMatch
}

type (
	store = iradix.Tree[[]routeEntry]
)

// TableMemory is an immutable routing table.
type TableMemory struct {
	store *store
}

// NewTableMemory creates an empty TableMemory.
func NewTableMemory() *TableMemory {
	return &TableMemory{
		store: iradix.New[[]routeEntry](),
	}
}

// Remember adds an InterceptorRoute and returns a new TableMemory.
// Duplicates are not detected and existing entries are not updated.
func (tm *TableMemory) Remember(ir *httpv1beta1.InterceptorRoute) *TableMemory {
	if ir == nil {
		return tm
	}
	ir = ir.DeepCopy()

	store := tm.store
	for _, rule := range ir.Spec.Rules {
		keys := newKeysFromRoutingRule(rule)

		for _, key := range keys {
			store = remember(store, key, ir, rule.Headers)
		}
	}

	return &TableMemory{
		store: store,
	}
}

// Route finds an InterceptorRoute matching hostname, path and headers.
// Tries exact match first the hostname, then filter by headers and find the most specific match
// then it moves to wildcards with similar header matching rule, finally catch-all.
func (tm *TableMemory) Route(hostname, path string, headers http.Header) *httpv1beta1.InterceptorRoute {
	// Try exact match
	key := NewKey(hostname, path)
	_, routeEntries, _ := tm.store.Root().LongestPrefix(key)

	// Filter header matches, the entries are already sorted by specificity so the first match is the most specific one
	for _, e := range routeEntries {
		if headersMatch(e.headers, headers) {
			return e.ir
		}
	}

	// Try wildcard matches (most specific to least specific)
	for _, wildcardName := range wildcardHostnames(hostname) {
		wildcardKey := NewKey(wildcardName, path)
		if ir := tm.tryWithKey(wildcardKey, headers); ir != nil {
			return ir
		}
	}

	// Try catch-all
	catchAllKey := NewKey(catchAllHostKey, path)
	return tm.tryWithKey(catchAllKey, headers)
}

// tryWithKey attempts to find an InterceptorRoute for the given key and headers.
func (tm *TableMemory) tryWithKey(key Key, headers http.Header) *httpv1beta1.InterceptorRoute {
	_, routeEntries, _ := tm.store.Root().LongestPrefix(key)
	for _, e := range routeEntries {
		if headersMatch(e.headers, headers) {
			return e.ir
		}
	}
	return nil
}

// headersMatch checks if the provided headers match the header requirements.
func headersMatch(matchers []httpv1beta1.HeaderMatch, reqHeaders http.Header) bool {
	if len(matchers) == 0 {
		// No header requirements, always match
		return true
	}

	for _, matcher := range matchers {
		reqHeaderValues := reqHeaders.Values(matcher.Name)
		if matcher.Value == nil && len(reqHeaderValues) == 0 {
			// Required header not present
			return false
		}

		if matcher.Value != nil && !slices.Contains(reqHeaderValues, *matcher.Value) {
			// Required header with value not present
			return false
		}
	}

	return true
}

// remember adds a route entry to the store for the given key, sorting by specificity.
// Duplicates are not detected and existing entries are not updated.
func remember(store *store, key Key, ir *httpv1beta1.InterceptorRoute, headers []httpv1beta1.HeaderMatch) *store {
	existing, found := store.Root().Get(key)
	re := routeEntry{ir: ir, headers: headers}
	if !found {
		// No existing entry, create a new one
		newStore, _, _ := store.Insert(key, []routeEntry{re})
		return newStore
	}

	existing = slices.Clone(existing)
	existing = append(existing, re)

	slices.SortFunc(existing, func(a, b routeEntry) int {
		// first by number of headers, more headers first
		if diff := cmp.Compare(len(b.headers), len(a.headers)); diff != 0 {
			return diff
		}

		// then by specificity of header values, more specific first
		if diff := cmp.Compare(headerSpecificity(b.headers), headerSpecificity(a.headers)); diff != 0 {
			return diff
		}

		// then by creation timestamp, older first
		if diff := a.ir.CreationTimestamp.Compare(b.ir.CreationTimestamp.Time); diff != 0 {
			return diff
		}

		// tiebreaker by namespace/name, lexicographically (descending)
		if diff := cmp.Compare(b.ir.Namespace, a.ir.Namespace); diff != 0 {
			return diff
		}

		return cmp.Compare(b.ir.Name, a.ir.Name)
	})

	newRadix, _, _ := store.Insert(key, existing)
	return newRadix
}

// headerSpecificity calculates how specific the headers are, putting more weight on headers with values.
func headerSpecificity(headers []httpv1beta1.HeaderMatch) int {
	specificity := 0
	for _, header := range headers {
		if header.Value != nil {
			specificity++
		}
	}
	return specificity
}
