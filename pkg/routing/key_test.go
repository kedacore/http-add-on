package routing

import (
	"net/http"
	"slices"
	"testing"

	httpv1beta1 "github.com/kedacore/http-add-on/operator/apis/http/v1beta1"
)

func TestNewKey(t *testing.T) {
	tests := map[string]struct {
		hostname string
		path     string
		want     string
	}{
		"empty":              {"", "", "/"},
		"host only":          {"example.com", "", "example.com/"},
		"path only":          {"", "/api/v1", "/api/v1/"},
		"host and path":      {"example.com", "/api/v1", "example.com/api/v1/"},
		"strips leading /":   {"", "///api", "/api/"},
		"strips trailing /":  {"", "api///", "/api/"},
		"strips both /":      {"", "///api///", "/api/"},
		"normalizes slashes": {"k8s.io", "//abc/def//", "k8s.io/abc/def/"},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got := NewKey(tt.hostname, tt.path)
			if got.String() != tt.want {
				t.Errorf("NewKey(%q, %q) = %q, want %q", tt.hostname, tt.path, got, tt.want)
			}
		})
	}
}

func TestNewKeyFromURL(t *testing.T) {
	t.Run("nil returns nil", func(t *testing.T) {
		if got := NewKeyFromURL(nil); got != nil {
			t.Errorf("got %q, want nil", got)
		}
	})

	t.Run("strips port and query", func(t *testing.T) {
		r, _ := http.NewRequest("GET", "https://k8s.io:443/api/v1?foo=bar#frag", nil)
		got := NewKeyFromURL(r.URL)
		if want := "k8s.io/api/v1/"; got.String() != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})
}

func TestNewKeyFromRequest(t *testing.T) {
	t.Run("nil returns nil", func(t *testing.T) {
		if got := NewKeyFromRequest(nil); got != nil {
			t.Errorf("got %q, want nil", got)
		}
	})

	t.Run("uses Host header over URL host", func(t *testing.T) {
		r, _ := http.NewRequest("GET", "http://url-host/path", nil)
		r.Host = "header-host"
		got := NewKeyFromRequest(r)
		if want := "header-host/path/"; got.String() != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})
}

func TestNewKeysFromRoutingRule(t *testing.T) {
	tests := map[string]struct {
		rule httpv1beta1.RoutingRule
		want []string
	}{
		"single host and path": {
			rule: httpv1beta1.RoutingRule{
				Hosts: []string{"example.com"},
				Paths: []httpv1beta1.PathMatch{{Value: "/api"}},
			},
			want: []string{"example.com/api/"},
		},
		"cartesian product": {
			rule: httpv1beta1.RoutingRule{
				Hosts: []string{"a.com", "b.com"},
				Paths: []httpv1beta1.PathMatch{{Value: "/x"}, {Value: "/y"}},
			},
			want: []string{"a.com/x/", "a.com/y/", "b.com/x/", "b.com/y/"},
		},
		"nil hosts defaults to catch-all": {
			rule: httpv1beta1.RoutingRule{
				Paths: []httpv1beta1.PathMatch{{Value: "/api"}},
			},
			want: []string{"*/api/"},
		},
		"nil paths defaults to root": {
			rule: httpv1beta1.RoutingRule{
				Hosts: []string{"example.com"},
			},
			want: []string{"example.com/"},
		},
		"empty rule defaults to catch-all root": {
			rule: httpv1beta1.RoutingRule{},
			want: []string{"*/"},
		},
		"star host becomes catch-all": {
			rule: httpv1beta1.RoutingRule{
				Hosts: []string{"*"},
				Paths: []httpv1beta1.PathMatch{{Value: "/api"}},
			},
			want: []string{"*/api/"},
		},
		"empty string host becomes catch-all": {
			rule: httpv1beta1.RoutingRule{
				Hosts: []string{""},
				Paths: []httpv1beta1.PathMatch{{Value: "/api"}},
			},
			want: []string{"*/api/"},
		},
		"host with port strips port": {
			rule: httpv1beta1.RoutingRule{
				Hosts: []string{"example.com:8080"},
				Paths: []httpv1beta1.PathMatch{{Value: "/api"}},
			},
			want: []string{"example.com/api/"},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			keys := newKeysFromRoutingRule(tt.rule)

			got := make([]string, len(keys))
			for i, k := range keys {
				got[i] = k.String()
			}

			slices.Sort(got)
			slices.Sort(tt.want)

			if !slices.Equal(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStripPort(t *testing.T) {
	tests := map[string]struct {
		host string
		want string
	}{
		"no port":        {"example.com", "example.com"},
		"with port":      {"example.com:8080", "example.com"},
		"empty":          {"", ""},
		"IPv6 with port": {"[::1]:8080", "::1"},
		"localhost port": {"localhost:3000", "localhost"},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got := stripPort(tt.host)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestWildcardHostnames(t *testing.T) {
	tests := map[string]struct {
		hostname string
		want     []string
	}{
		"multi-level":  {"a.b.example.com", []string{"*.b.example.com", "*.example.com", "*.com"}},
		"single-label": {"localhost", nil},
		"empty":        {"", nil},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got := wildcardHostnames(tt.hostname)
			if !slices.Equal(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}
