package routing

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"k8s.io/utils/ptr"

	httpv1beta1 "github.com/kedacore/http-add-on/operator/apis/http/v1beta1"
)

func TestMatchRoutingRule(t *testing.T) {
	headerValue := "header-value"

	tests := map[string]struct {
		rule    httpv1beta1.RoutingRule
		path    string
		host    string
		headers map[string]string
		want    bool
	}{
		"empty rule matches everything": {
			rule: httpv1beta1.RoutingRule{},
			path: "/anything",
			host: "any.host",
			want: true,
		},
		"path only": {
			rule: httpv1beta1.RoutingRule{Paths: []httpv1beta1.PathMatch{{Value: "/healthz"}}},
			path: "/healthz",
			want: true,
		},
		"path no match": {
			rule: httpv1beta1.RoutingRule{Paths: []httpv1beta1.PathMatch{{Value: "/healthz"}}},
			path: "/other",
			want: false,
		},
		"host only match": {
			rule: httpv1beta1.RoutingRule{Hosts: []string{"example.com"}},
			host: "example.com",
			want: true,
		},
		"host only miss": {
			rule: httpv1beta1.RoutingRule{Hosts: []string{"example.com"}},
			host: "example.org",
			want: false,
		},
		"header only match": {
			rule: httpv1beta1.RoutingRule{
				Headers: []httpv1beta1.HeaderMatch{{Name: "Custom-Header", Value: &headerValue}},
			},
			headers: map[string]string{"Custom-Header": headerValue},
			want:    true,
		},
		"header only miss": {
			rule: httpv1beta1.RoutingRule{
				Headers: []httpv1beta1.HeaderMatch{{Name: "Custom-Header", Value: &headerValue}},
			},
			headers: map[string]string{"Custom-Header": "not matching"},
			want:    false,
		},
		"all fields match": {
			rule: httpv1beta1.RoutingRule{
				Hosts:   []string{"app.example.com"},
				Paths:   []httpv1beta1.PathMatch{{Value: "/api"}},
				Headers: []httpv1beta1.HeaderMatch{{Name: "Custom-Header", Value: &headerValue}},
			},
			host:    "app.example.com",
			path:    "/api/v1",
			headers: map[string]string{"Custom-Header": headerValue},
			want:    true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			path := tc.path
			if path == "" {
				path = "/"
			}

			req := httptest.NewRequest(http.MethodGet, path, nil)
			if tc.host != "" {
				req.Host = tc.host
			}
			for k, v := range tc.headers {
				req.Header.Set(k, v)
			}

			got := MatchRoutingRule(req, tc.rule)
			if got != tc.want {
				t.Fatalf("MatchRoutingRule() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestRequestHost(t *testing.T) {
	tests := map[string]struct {
		reqHost string
		urlHost string
		want    string
	}{
		"strips port": {
			reqHost: "example.com:8080",
			want:    "example.com",
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

func TestMatchAnyHost(t *testing.T) {
	tests := map[string]struct {
		host  string
		hosts []string
		want  bool
	}{
		"exact match":                   {host: "a.com", hosts: []string{"a.com"}, want: true},
		"no match":                      {host: "b.com", hosts: []string{"a.com"}, want: false},
		"wildcard match":                {host: "x.a.com", hosts: []string{"*.a.com"}, want: true},
		"wildcard rejects bare domain":  {host: "a.com", hosts: []string{"*.a.com"}, want: false},
		"wildcard matches nested":       {host: "x.y.a.com", hosts: []string{"*.a.com"}, want: true},
		"catch-all star":                {host: "anything", hosts: []string{"*"}, want: true},
		"catch-all empty":               {host: "anything", hosts: []string{""}, want: true},
		"multiple hosts first matches":  {host: "a.com", hosts: []string{"a.com", "b.com"}, want: true},
		"multiple hosts second matches": {host: "b.com", hosts: []string{"a.com", "b.com"}, want: true},
		"multiple hosts none match":     {host: "c.com", hosts: []string{"a.com", "b.com"}, want: false},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := MatchAnyHost(tc.host, tc.hosts)
			if got != tc.want {
				t.Fatalf("MatchAnyHost(%q, %v) = %v, want %v", tc.host, tc.hosts, got, tc.want)
			}
		})
	}
}

func TestMatchAnyPath(t *testing.T) {
	tests := map[string]struct {
		path  string
		paths []httpv1beta1.PathMatch
		want  bool
	}{
		"exact match": {
			path:  "/healthz",
			paths: []httpv1beta1.PathMatch{{Value: "/healthz"}},
			want:  true,
		},
		"prefix match": {
			path:  "/api/v1",
			paths: []httpv1beta1.PathMatch{{Value: "/api"}},
			want:  true,
		},
		"no partial word match": {
			path:  "/healthz",
			paths: []httpv1beta1.PathMatch{{Value: "/health"}},
			want:  false,
		},
		"root matches everything": {
			path:  "/anything",
			paths: []httpv1beta1.PathMatch{{Value: "/"}},
			want:  true,
		},
		"trailing slash ignored": {
			path:  "/healthz",
			paths: []httpv1beta1.PathMatch{{Value: "/healthz/"}},
			want:  true,
		},
		"multiple paths second matches": {
			path:  "/readyz",
			paths: []httpv1beta1.PathMatch{{Value: "/healthz"}, {Value: "/readyz"}},
			want:  true,
		},
		"multiple paths none match": {
			path:  "/other",
			paths: []httpv1beta1.PathMatch{{Value: "/healthz"}, {Value: "/readyz"}},
			want:  false,
		},
		"request trailing slash": {
			path:  "/healthz/",
			paths: []httpv1beta1.PathMatch{{Value: "/healthz"}},
			want:  true,
		},
		"empty path matches everything": {
			path:  "/anything",
			paths: []httpv1beta1.PathMatch{{Value: ""}},
			want:  true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := MatchAnyPath(tc.path, tc.paths)
			if got != tc.want {
				t.Fatalf("MatchAnyPath(%q, ...) = %v, want %v", tc.path, got, tc.want)
			}
		})
	}
}

func TestMatchHeaders(t *testing.T) {
	matchers := []httpv1beta1.HeaderMatch{
		{Name: "Required-Header", Value: ptr.To("expected")},
		{Name: "Key-Only-Header"},
	}

	tests := map[string]struct {
		headers map[string][]string
		want    bool
	}{
		"matches exact header": {
			headers: map[string][]string{"Required-Header": {"expected"}, "Key-Only-Header": {"any-value"}},
			want:    true,
		},
		"missing required but has key-only header": {
			headers: map[string][]string{"Key-Only-Header": {"any-value"}},
			want:    false,
		},
		"missing key-only header": {
			headers: map[string][]string{"Required-Header": {"expected"}},
			want:    false,
		},
		"wrong value for required header": {
			headers: map[string][]string{"Required-Header": {"wrong"}, "Key-Only-Header": {"any-value"}},
			want:    false,
		},
		"empty string as value": {
			headers: map[string][]string{"Required-Header": {"expected"}, "Key-Only-Header": {""}},
			want:    true,
		},
		"no headers provided": {
			headers: map[string][]string{},
			want:    false,
		},
		"extra headers provided": {
			headers: map[string][]string{"Required-Header": {"expected"}, "Key-Only-Header": {"any-value"}, "Extra-Header": {"extra"}},
			want:    true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			result := MatchHeaders(matchers, tc.headers)
			if result != tc.want {
				t.Errorf("MatchHeaders()=%v, want=%v", result, tc.want)
			}
		})
	}

	t.Run("empty matchers match everything", func(t *testing.T) {
		result := MatchHeaders(nil, map[string][]string{"Any-Header": {"any-value"}})
		if !result {
			t.Error("expected empty matchers to match")
		}
	})

	emptyHeaderValueMatchers := []httpv1beta1.HeaderMatch{
		{Name: "Empty-Value-Header", Value: ptr.To("")},
	}

	t.Run("matches empty string header value", func(t *testing.T) {
		result := MatchHeaders(emptyHeaderValueMatchers, map[string][]string{"Empty-Value-Header": {""}})
		if !result {
			t.Error("expected headers to match")
		}
	})

	t.Run("does not match non-empty string for empty value header", func(t *testing.T) {
		result := MatchHeaders(emptyHeaderValueMatchers, map[string][]string{"Empty-Value-Header": {"non-empty"}})
		if result {
			t.Error("expected headers not to match")
		}
	})
}
