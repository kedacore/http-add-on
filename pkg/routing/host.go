package routing

import (
	"net"
	"net/http"
	"strings"
)

// RequestHost returns the normalized hostname of the request.
// Falls back to r.URL.Host when r.Host is empty.
func RequestHost(r *http.Request) string {
	host := r.Host
	if host == "" && r.URL != nil {
		host = r.URL.Host
	}
	return normalizeHostname(host)
}

// normalizeHostname removes the port and the brackets of an IPv6 literal, and
// lowercases the result since hostnames are case-insensitive (RFC 9110,
// section 4.2.3). "Example.com:8080" yields "example.com", and both
// "[2001:DB8::1]" and "[2001:db8::1]:8080" yield "2001:db8::1".
func normalizeHostname(host string) string {
	if hostname, _, err := net.SplitHostPort(host); err == nil {
		host = hostname
	} else if strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") {
		host = host[1 : len(host)-1]
	}
	return strings.ToLower(host)
}
