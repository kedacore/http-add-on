package routing

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"

	httpv1alpha1 "github.com/kedacore/http-add-on/operator/apis/http/v1alpha1"
)

// catchAllHostKey is the internal routing key for catch-all host matching.
const catchAllHostKey = "*"

type (
	Key  []byte
	Keys []Key
)

var _ fmt.Stringer = (*Key)(nil)

func (k Key) String() string {
	return string(k)
}

// NewKey creates a routing key from hostname (without port) and path.
func NewKey(hostname string, path string) Key {
	path = strings.Trim(path, "/")

	var b strings.Builder
	b.Grow(len(hostname) + 1 + len(path) + 1)
	b.WriteString(hostname)
	b.WriteByte('/')
	b.WriteString(path)
	if path != "" {
		b.WriteByte('/')
	}
	return []byte(b.String())
}

// NewKeyFromURL creates a routing key from a URL.
func NewKeyFromURL(url *url.URL) Key {
	if url == nil {
		return nil
	}

	return NewKey(url.Hostname(), url.Path)
}

// NewKeyFromRequest creates a routing key from an HTTP request.
func NewKeyFromRequest(req *http.Request) Key {
	if req == nil {
		return nil
	}

	reqURL := req.URL
	if reqURL == nil {
		return nil
	}

	keyURL := *reqURL
	if reqHost := req.Host; reqHost != "" {
		keyURL.Host = reqHost
	}

	return NewKeyFromURL(&keyURL)
}

// NewKeysFromHTTPSO creates routing keys from an HTTPScaledObject.
func NewKeysFromHTTPSO(httpso *httpv1alpha1.HTTPScaledObject) Keys {
	if httpso == nil {
		return nil
	}
	spec := httpso.Spec

	hosts := spec.Hosts
	if hosts == nil {
		hosts = []string{""}
	}
	hostsSize := len(hosts)

	pathPrefixes := spec.PathPrefixes
	if pathPrefixes == nil {
		pathPrefixes = []string{""}
	}
	pathPrefixesSize := len(pathPrefixes)

	keysSize := hostsSize * pathPrefixesSize
	keys := make([]Key, 0, keysSize)
	for _, host := range hosts {
		hostname := stripPort(host)
		if isCatchAllHostname(hostname) {
			hostname = catchAllHostKey
		}
		for _, pathPrefix := range pathPrefixes {
			key := NewKey(hostname, pathPrefix)
			keys = append(keys, key)
		}
	}

	return keys
}

// stripPort removes the port from a host string.
func stripPort(host string) string {
	if host == "" {
		return ""
	}
	if hostname, _, err := net.SplitHostPort(host); err == nil {
		return hostname
	}
	return host
}

// wildcardHostnames returns all wildcard patterns for a hostname,
// ordered from most specific to least specific.
// "foo.example.com" -> ["*.example.com", "*.com"]
// "localhost"       -> []  (single-label, no wildcards)
// ""                -> []  (empty, no wildcards)
func wildcardHostnames(hostname string) []string {
	if hostname == "" {
		return nil
	}

	// Count dots to pre-allocate exact capacity
	dotCount := strings.Count(hostname, ".")
	if dotCount == 0 {
		return nil // Single-label hostname
	}

	wildcards := make([]string, 0, dotCount)
	remaining := hostname

	for {
		// Use Index instead of SplitN to avoid []string allocation
		idx := strings.Index(remaining, ".")
		if idx == -1 {
			break
		}
		remaining = remaining[idx+1:]
		wildcards = append(wildcards, "*."+remaining)
	}

	return wildcards
}

// isCatchAllHostname returns true if hostname is a catch-all wildcard.
func isCatchAllHostname(hostname string) bool {
	return hostname == "*" || hostname == ""
}
