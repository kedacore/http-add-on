package main

import (
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	httpv1alpha1 "github.com/kedacore/http-add-on/operator/apis/http/v1alpha1"
)

// RoutingTable provides lock-free route lookups.
//
// Reads are wait-free (a single atomic.Pointer load). The table is rebuilt
// atomically on every HTTPScaledObject change. The matching algorithm mirrors
// the Go/Rust implementations: exact host -> wildcard hosts -> catch-all "*",
// each combined with longest-path-prefix and header filtering.
type RoutingTable struct {
	memory atomic.Pointer[tableMemory]
	synced atomic.Bool
}

// RouteInfo is the pre-computed result of a route match. All strings are
// allocated once at table-build time, so the hot-path lookup returns
// references without any per-request allocation.
type RouteInfo struct {
	QueueKey              string
	Authority             string        // "service.namespace:port"
	ServiceKey            string        // "namespace/service" for endpoint lookups
	ConditionWaitTimeout  time.Duration // 0 means use global default
	ResponseHeaderTimeout time.Duration // 0 means use global default
	FailoverAuthority     string        // "service.namespace:port" for failover target
	FailoverTimeout       time.Duration
	HasFailover           bool
}

// NewRoutingTable creates a new empty routing table.
func NewRoutingTable() *RoutingTable {
	rt := &RoutingTable{}
	rt.memory.Store(&tableMemory{})
	return rt
}

// Route performs a lock-free route lookup for the given request.
// Returns nil if no route matches.
//
//go:nosplit
func (rt *RoutingTable) Route(host, path string, headers http.Header) *RouteInfo {
	return rt.memory.Load().lookup(host, path, headers)
}

// PortResolver resolves a named service port to a numeric port.
// Returns 0 if the port cannot be resolved.
type PortResolver func(namespace, service, portName string) int32

// Rebuild atomically swaps in a new routing table built from the given CRDs.
// If resolvePort is non-nil, named ports (spec.scaleTargetRef.portName) are
// resolved via a Service lookup.
func (rt *RoutingTable) Rebuild(objects []httpv1alpha1.HTTPScaledObject, resolvePort PortResolver) {
	rt.memory.Store(buildTableMemory(objects, resolvePort))
	rt.synced.Store(true)
}

// HasSynced returns true once the table has been built at least once.
func (rt *RoutingTable) HasSynced() bool {
	return rt.synced.Load()
}

// ---------------------------------------------------------------------------
// Internal: immutable snapshot
// ---------------------------------------------------------------------------

// tableMemory is an immutable snapshot of all routes, swapped atomically.
type tableMemory struct {
	// hostname -> entries sorted by (path-prefix length desc, header count desc)
	routes map[string][]routeEntry
}

type routeEntry struct {
	pathPrefix     string
	headerMatchers []headerMatcher
	info           RouteInfo // pre-computed, returned by pointer
}

type headerMatcher struct {
	name  string // lowercased
	value string // empty = presence-only check
}

// ---------------------------------------------------------------------------
// Build
// ---------------------------------------------------------------------------

func buildTableMemory(objects []httpv1alpha1.HTTPScaledObject, resolvePort PortResolver) *tableMemory {
	routes := make(map[string][]routeEntry, len(objects))

	for i := range objects {
		httpso := &objects[i]
		namespace := httpso.Namespace
		if namespace == "" {
			namespace = defaultNamespace
		}
		name := httpso.Name
		spec := &httpso.Spec

		// Resolve port: prefer explicit port, then named port lookup, then 80
		port := spec.ScaleTargetRef.Port
		if port == 0 && spec.ScaleTargetRef.PortName != "" && resolvePort != nil {
			port = resolvePort(namespace, spec.ScaleTargetRef.Service, spec.ScaleTargetRef.PortName)
		}
		if port == 0 {
			port = 80
		}

		// Failover
		var failoverAuthority string
		var failoverTimeout time.Duration
		hasFailover := false
		if f := spec.ColdStartTimeoutFailoverRef; f != nil {
			fp := f.Port
			if fp == 0 {
				fp = 80
			}
			failoverAuthority = f.Service + "." + namespace + ":" + itoa(fp)
			ts := f.TimeoutSeconds
			if ts <= 0 {
				ts = 30
			}
			failoverTimeout = time.Duration(ts) * time.Second
			hasFailover = true
		}

		// Timeouts
		var conditionWaitTimeout, responseHeaderTimeout time.Duration
		if t := spec.Timeouts; t != nil {
			conditionWaitTimeout = t.ConditionWait.Duration
			responseHeaderTimeout = t.ResponseHeader.Duration
		}

		// Header matchers (lowercased for case-insensitive matching)
		matchers := make([]headerMatcher, 0, len(spec.Headers))
		for _, h := range spec.Headers {
			v := ""
			if h.Value != nil {
				v = *h.Value
			}
			matchers = append(matchers, headerMatcher{
				name:  strings.ToLower(h.Name),
				value: v,
			})
		}

		// Hosts and path prefixes (defaults)
		hosts := spec.Hosts
		if len(hosts) == 0 {
			hosts = []string{"*"}
		}
		pathPrefixes := spec.PathPrefixes
		if len(pathPrefixes) == 0 {
			pathPrefixes = []string{"/"}
		}

		httpsoKey := namespace + "/" + name
		serviceKey := namespace + "/" + spec.ScaleTargetRef.Service
		authority := spec.ScaleTargetRef.Service + "." + namespace + ":" + itoa(port)

		for _, host := range hosts {
			for _, prefix := range pathPrefixes {
				entry := routeEntry{
					pathPrefix:     normalizePath(prefix),
					headerMatchers: matchers,
					info: RouteInfo{
						QueueKey:              httpsoKey,
						Authority:             authority,
						ServiceKey:            serviceKey,
						ConditionWaitTimeout:  conditionWaitTimeout,
						ResponseHeaderTimeout: responseHeaderTimeout,
						FailoverAuthority:     failoverAuthority,
						FailoverTimeout:       failoverTimeout,
						HasFailover:           hasFailover,
					},
				}
				routes[host] = append(routes[host], entry)
			}
		}
	}

	// Sort each host's entries: longest prefix first, then most headers.
	for _, entries := range routes {
		sortRouteEntries(entries)
	}

	return &tableMemory{routes: routes}
}

// sortRouteEntries sorts by path prefix length descending, then header count descending.
func sortRouteEntries(entries []routeEntry) {
	// Insertion sort — entry lists are tiny (usually 1-3 items).
	for i := 1; i < len(entries); i++ {
		for j := i; j > 0; j-- {
			if compareEntries(&entries[j], &entries[j-1]) {
				entries[j], entries[j-1] = entries[j-1], entries[j]
			} else {
				break
			}
		}
	}
}

func compareEntries(a, b *routeEntry) bool {
	if len(a.pathPrefix) != len(b.pathPrefix) {
		return len(a.pathPrefix) > len(b.pathPrefix)
	}
	return len(a.headerMatchers) > len(b.headerMatchers)
}

// ---------------------------------------------------------------------------
// Lookup
// ---------------------------------------------------------------------------

func (tm *tableMemory) lookup(host, path string, headers http.Header) *RouteInfo {
	hostStripped := stripPort(host)
	if path == "" {
		path = "/"
	}

	// 1. Exact hostname
	if info := tm.tryMatch(hostStripped, path, headers); info != nil {
		return info
	}

	// 2. Wildcard hostnames: *.example.com -> *.com
	parts := strings.Split(hostStripped, ".")
	for i := 1; i < len(parts); i++ {
		wildcard := "*." + strings.Join(parts[i:], ".")
		if info := tm.tryMatch(wildcard, path, headers); info != nil {
			return info
		}
	}

	// 3. Catch-all
	return tm.tryMatch("*", path, headers)
}

func (tm *tableMemory) tryMatch(hostname, path string, headers http.Header) *RouteInfo {
	entries, ok := tm.routes[hostname]
	if !ok {
		return nil
	}

	// Normalize the request path with a trailing "/" so that segment-boundary
	// matching works: "/api/v1/" starts with "/api/" but "/api2/" does not.
	// The root "/" stays unchanged.
	matchPath := path
	if matchPath != "/" && !strings.HasSuffix(matchPath, "/") {
		matchPath += "/"
	}

	// Entries are pre-sorted; the first matching entry wins.
	for i := range entries {
		e := &entries[i]
		if !strings.HasPrefix(matchPath, e.pathPrefix) {
			continue
		}
		if !headersMatch(e.headerMatchers, headers) {
			continue
		}
		return &e.info
	}
	return nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func headersMatch(matchers []headerMatcher, headers http.Header) bool {
	for i := range matchers {
		m := &matchers[i]
		vals := headers.Values(m.name)
		if len(vals) == 0 {
			return false
		}
		if m.value != "" {
			found := false
			for _, v := range vals {
				if v == m.value {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
	}
	return true
}

// normalizePath ensures a path prefix starts and ends with "/".
// The trailing "/" guarantees segment-boundary matching: "/api/" matches
// "/api/v1" but not "/api2". The root "/" is left as-is.
func normalizePath(prefix string) string {
	if prefix == "" || prefix == "/" {
		return "/"
	}
	if prefix[0] != '/' {
		prefix = "/" + prefix
	}
	if prefix[len(prefix)-1] != '/' {
		prefix += "/"
	}
	return prefix
}

func stripPort(host string) string {
	// Preserve IPv6 addresses like [::1]:8080
	if len(host) > 0 && host[0] == '[' {
		return host
	}
	if i := strings.LastIndexByte(host, ':'); i >= 0 {
		return host[:i]
	}
	return host
}

func itoa(n int32) string {
	return strconv.Itoa(int(n))
}
