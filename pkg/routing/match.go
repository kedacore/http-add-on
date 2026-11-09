package routing

import (
	"net/http"
	"slices"
	"strings"

	httpv1beta1 "github.com/kedacore/http-add-on/operator/apis/http/v1beta1"
)

// MatchRoutingRule reports whether the request matches the given rule.
// Empty fields match everything. Hosts and paths use OR semantics; headers use AND semantics.
func MatchRoutingRule(r *http.Request, rule httpv1beta1.RoutingRule) bool {
	if len(rule.Hosts) > 0 && !MatchAnyHost(RequestHost(r), rule.Hosts) {
		return false
	}
	if len(rule.Paths) > 0 && !MatchAnyPath(r.URL.Path, rule.Paths) {
		return false
	}
	if len(rule.Headers) > 0 && !MatchHeaders(rule.Headers, r.Header) {
		return false
	}
	return true
}

// MatchHeaders reports whether reqHeaders satisfies all matchers (AND semantics).
// A matcher with a nil Value requires the header to be present with any value.
func MatchHeaders(matchers []httpv1beta1.HeaderMatch, reqHeaders http.Header) bool {
	for _, m := range matchers {
		vals := reqHeaders.Values(m.Name)
		if m.Value == nil && len(vals) == 0 {
			return false
		}
		if m.Value != nil && !slices.Contains(vals, *m.Value) {
			return false
		}
	}
	return true
}

// MatchAnyHost reports whether reqHost matches any of the host patterns.
// Supports exact match, wildcard prefix ("*.example.com"), and catch-all ("*" or "").
// Hostnames are compared case-insensitively.
func MatchAnyHost(reqHost string, hosts []string) bool {
	reqHost = normalizeHostname(reqHost)
	for _, h := range hosts {
		h = normalizeHostname(h)
		if h == "*" || h == "" {
			return true
		}
		if strings.HasPrefix(h, "*.") {
			suffix := h[1:]
			if strings.HasSuffix(reqHost, suffix) {
				return true
			}
			continue
		}
		if reqHost == h {
			return true
		}
	}
	return false
}

// MatchAnyPath reports whether reqPath prefix-matches any of the path patterns.
// A root path ("/" or "") matches everything.
func MatchAnyPath(reqPath string, paths []httpv1beta1.PathMatch) bool {
	norm := normalizePath(reqPath)
	for _, p := range paths {
		prefix := normalizePath(p.Value)
		if prefix == "" {
			return true
		}
		if norm == prefix || strings.HasPrefix(norm, prefix+"/") {
			return true
		}
	}
	return false
}

func normalizePath(p string) string {
	return strings.TrimRight(strings.TrimLeft(p, "/"), "/")
}
