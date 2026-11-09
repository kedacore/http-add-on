package routing

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequestHost(t *testing.T) {
	tests := map[string]struct {
		reqHost string
		urlHost string
		want    string
	}{
		"no port": {
			reqHost: "example.com",
			want:    "example.com",
		},
		"strips port": {
			reqHost: "example.com:8080",
			want:    "example.com",
		},
		"empty": {
			want: "",
		},
		"lowercases": {
			reqHost: "Example.COM:8080",
			want:    "example.com",
		},
		"IPv6 without port": {
			reqHost: "[2001:db8::1]",
			want:    "2001:db8::1",
		},
		"IPv6 with port": {
			reqHost: "[2001:db8::1]:8080",
			want:    "2001:db8::1",
		},
		"IPv6 lowercases": {
			reqHost: "[2001:DB8::1]",
			want:    "2001:db8::1",
		},
		"falls back to URL host": {
			urlHost: "example.com:443",
			want:    "example.com",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Host = tc.reqHost
			if tc.urlHost != "" {
				req.URL.Host = tc.urlHost
			}

			got := RequestHost(req)
			if got != tc.want {
				t.Fatalf("RequestHost() = %q, want %q", got, tc.want)
			}
		})
	}
}
