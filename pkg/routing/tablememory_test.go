package routing

import (
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	httpv1beta1 "github.com/kedacore/http-add-on/operator/apis/http/v1beta1"
)

func TestRemember(t *testing.T) {
	t.Run("stores and routes single object", func(t *testing.T) {
		ir := &httpv1beta1.InterceptorRoute{
			ObjectMeta: metav1.ObjectMeta{Name: "test"},
			Spec: httpv1beta1.InterceptorRouteSpec{
				Rules: []httpv1beta1.RoutingRule{{Hosts: []string{"example.com"}}},
			},
		}
		runRouteTest(t, []*httpv1beta1.InterceptorRoute{ir}, "example.com", "", ir)
	})

	t.Run("retains other objects", func(t *testing.T) {
		ir1 := &httpv1beta1.InterceptorRoute{
			ObjectMeta: metav1.ObjectMeta{Name: "first"},
			Spec: httpv1beta1.InterceptorRouteSpec{
				Rules: []httpv1beta1.RoutingRule{{Hosts: []string{"first.com"}}},
			},
		}
		ir2 := &httpv1beta1.InterceptorRoute{
			ObjectMeta: metav1.ObjectMeta{Name: "second"},
			Spec: httpv1beta1.InterceptorRouteSpec{
				Rules: []httpv1beta1.RoutingRule{{Hosts: []string{"second.com"}}},
			},
		}

		tm := NewTableMemory().Remember(ir1).Remember(ir2)

		route1 := tm.Route("first.com", "", nil)
		if route1 == nil || route1.Name != "first" {
			name := "<nil>"
			if route1 != nil {
				name = route1.Name
			}
			t.Errorf("name=%q, want=%q", name, "first")
		}
		route2 := tm.Route("second.com", "", nil)
		if route2 == nil || route2.Name != "second" {
			name := "<nil>"
			if route2 != nil {
				name = route2.Name
			}
			t.Errorf("name=%q, want=%q", name, "second")
		}
	})

	t.Run("deep copies input", func(t *testing.T) {
		ir := &httpv1beta1.InterceptorRoute{
			ObjectMeta: metav1.ObjectMeta{Name: "test"},
			Spec: httpv1beta1.InterceptorRouteSpec{
				Rules: []httpv1beta1.RoutingRule{{Hosts: []string{"example.com"}}},
			},
		}

		tm := NewTableMemory().Remember(ir)

		// Modify original after storing
		ir.Spec.Rules[0].Hosts[0] = "modified.com"

		// Should still route to original host, not modified one
		if tm.Route("example.com", "", nil) == nil {
			t.Error("expected route for original host")
		}
		if tm.Route("modified.com", "", nil) != nil {
			t.Error("expected no route for modified host")
		}
	})
}

func TestRememberOldestWins(t *testing.T) {
	now := time.Now()

	older := &httpv1beta1.InterceptorRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "older",
			CreationTimestamp: metav1.NewTime(now.Add(-time.Hour)),
		},
		Spec: httpv1beta1.InterceptorRouteSpec{
			Rules: []httpv1beta1.RoutingRule{{Hosts: []string{"example.com"}}},
		},
	}
	newer := &httpv1beta1.InterceptorRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "newer",
			CreationTimestamp: metav1.NewTime(now),
		},
		Spec: httpv1beta1.InterceptorRouteSpec{
			Rules: []httpv1beta1.RoutingRule{{Hosts: []string{"example.com"}}},
		},
	}

	t.Run("older object wins when added first", func(t *testing.T) {
		tm := NewTableMemory().Remember(older).Remember(newer)

		route := tm.Route("example.com", "", nil)
		if route == nil {
			t.Fatal("no route matched")
		}
		if route.Name != "older" {
			t.Errorf("name=%q, want=%q", route.Name, "older")
		}
	})

	t.Run("older object wins when added second", func(t *testing.T) {
		tm := NewTableMemory().Remember(newer).Remember(older)

		route := tm.Route("example.com", "", nil)
		if route == nil {
			t.Fatal("no route matched")
		}
		if route.Name != "older" {
			t.Errorf("name=%q, want=%q", route.Name, "older")
		}
	})

	t.Run("older object wins with headers", func(t *testing.T) {
		olderWithHeaders := &httpv1beta1.InterceptorRoute{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "older-with-headers",
				CreationTimestamp: metav1.NewTime(now.Add(-time.Hour)),
			},
			Spec: httpv1beta1.InterceptorRouteSpec{
				Rules: []httpv1beta1.RoutingRule{{
					Hosts:   []string{"example.com"},
					Headers: []httpv1beta1.HeaderMatch{{Name: "X-Custom-Header", Value: ptr.To("value")}},
				}},
			},
		}
		newerWithHeaders := &httpv1beta1.InterceptorRoute{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "newer-with-headers",
				CreationTimestamp: metav1.NewTime(now),
			},
			Spec: httpv1beta1.InterceptorRouteSpec{
				Rules: []httpv1beta1.RoutingRule{{
					Hosts:   []string{"example.com"},
					Headers: []httpv1beta1.HeaderMatch{{Name: "X-Custom-Header", Value: ptr.To("value")}},
				}},
			},
		}

		tm := NewTableMemory().Remember(newerWithHeaders).Remember(olderWithHeaders)

		route := tm.Route("example.com", "", map[string][]string{"X-Custom-Header": {"value"}})
		if route == nil {
			t.Fatal("no route matched")
		}
		if route.Name != "older-with-headers" {
			t.Errorf("name=%q, want=%q", route.Name, "older-with-headers")
		}
	})
}

func TestRouteWithHeaders(t *testing.T) {
	fooIR := &httpv1beta1.InterceptorRoute{
		ObjectMeta: metav1.ObjectMeta{Name: "foo"},
		Spec: httpv1beta1.InterceptorRouteSpec{
			Rules: []httpv1beta1.RoutingRule{{
				Hosts:   []string{"example.com"},
				Paths:   []httpv1beta1.PathMatch{{Value: "/api/"}},
				Headers: []httpv1beta1.HeaderMatch{{Name: "X-Custom-Header", Value: ptr.To("foo")}},
			}},
		},
	}
	differentPathIR := &httpv1beta1.InterceptorRoute{
		ObjectMeta: metav1.ObjectMeta{Name: "different-path"},
		Spec: httpv1beta1.InterceptorRouteSpec{
			Rules: []httpv1beta1.RoutingRule{{
				Hosts:   []string{"example.com"},
				Paths:   []httpv1beta1.PathMatch{{Value: "/other/"}},
				Headers: []httpv1beta1.HeaderMatch{{Name: "X-Custom-Header", Value: ptr.To("foo")}},
			}},
		},
	}
	barIR := &httpv1beta1.InterceptorRoute{
		ObjectMeta: metav1.ObjectMeta{Name: "bar"},
		Spec: httpv1beta1.InterceptorRouteSpec{
			Rules: []httpv1beta1.RoutingRule{{
				Hosts:   []string{"example.com"},
				Paths:   []httpv1beta1.PathMatch{{Value: "/api/"}},
				Headers: []httpv1beta1.HeaderMatch{{Name: "X-Custom-Header", Value: ptr.To("bar")}},
			}},
		},
	}
	bazIR := &httpv1beta1.InterceptorRoute{
		ObjectMeta: metav1.ObjectMeta{Name: "baz"},
		Spec: httpv1beta1.InterceptorRouteSpec{
			Rules: []httpv1beta1.RoutingRule{{
				Hosts: []string{"example.com"},
				Paths: []httpv1beta1.PathMatch{{Value: "/api/"}},
			}},
		},
	}
	headerKeyIR := &httpv1beta1.InterceptorRoute{
		ObjectMeta: metav1.ObjectMeta{Name: "header-key-only"},
		Spec: httpv1beta1.InterceptorRouteSpec{
			Rules: []httpv1beta1.RoutingRule{{
				Hosts:   []string{"example.com"},
				Paths:   []httpv1beta1.PathMatch{{Value: "/api/"}},
				Headers: []httpv1beta1.HeaderMatch{{Name: "X-Custom-Header"}},
			}},
		},
	}
	manyHeadersIR := &httpv1beta1.InterceptorRoute{
		ObjectMeta: metav1.ObjectMeta{Name: "many-headers"},
		Spec: httpv1beta1.InterceptorRouteSpec{
			Rules: []httpv1beta1.RoutingRule{{
				Hosts: []string{"example.com"},
				Paths: []httpv1beta1.PathMatch{{Value: "/api/"}},
				Headers: []httpv1beta1.HeaderMatch{
					{Name: "X-Custom-Header", Value: ptr.To("foo")},
					{Name: "X-Another-Header", Value: ptr.To("baz")},
				},
			}},
		},
	}
	manyHeadersButKeyOnlyIR := &httpv1beta1.InterceptorRoute{
		ObjectMeta: metav1.ObjectMeta{Name: "many-headers-but-key-only"},
		Spec: httpv1beta1.InterceptorRouteSpec{
			Rules: []httpv1beta1.RoutingRule{{
				Hosts: []string{"example.com"},
				Paths: []httpv1beta1.PathMatch{{Value: "/api/"}},
				Headers: []httpv1beta1.HeaderMatch{
					{Name: "X-Custom-Header"},
					{Name: "X-Another-Header", Value: ptr.To("baz")},
				},
			}},
		},
	}

	tm := NewTableMemory().Remember(fooIR)
	tm = tm.Remember(differentPathIR)
	tm = tm.Remember(barIR)
	tm = tm.Remember(bazIR)
	tm = tm.Remember(headerKeyIR)
	tm = tm.Remember(manyHeadersIR)
	tm = tm.Remember(manyHeadersButKeyOnlyIR)

	tests := []struct {
		name    string
		headers map[string][]string
		path    string
		want    string // expected Name, or "" for nil
	}{
		{
			name:    "matches foo header",
			headers: map[string][]string{"X-Custom-Header": {"foo"}},
			path:    "/api/v1/resource",
			want:    "foo",
		},
		{
			name:    "matches bar header",
			headers: map[string][]string{"X-Custom-Header": {"bar"}},
			path:    "/api/v1/resource",
			want:    "bar",
		},
		{
			name:    "random headers returns baz (no header match) because it is the most specific",
			headers: map[string][]string{"X-Other-Header": {"value"}},
			path:    "/api/v1/resource",
			want:    "baz",
		},
		{
			name:    "no header returns baz (no header requirement)",
			headers: map[string][]string{},
			path:    "/api/v1/resource",
			want:    "baz",
		},
		{
			name:    "different path returns correct object",
			headers: map[string][]string{"X-Custom-Header": {"foo"}},
			path:    "/other/resource",
			want:    "different-path",
		},
		{
			name:    "header key only matches any value",
			headers: map[string][]string{"X-Custom-Header": {"any-value"}},
			path:    "/api/v1/resource",
			want:    "header-key-only",
		},
		{
			name:    "no match returns nil",
			headers: map[string][]string{"X-Custom-Header": {"non-matching"}},
			path:    "/other/resource",
			want:    "",
		},
		{
			name:    "matches many headers",
			headers: map[string][]string{"X-Custom-Header": {"foo"}, "X-Another-Header": {"baz"}},
			path:    "/api/v1/resource",
			want:    "many-headers",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			route := tm.Route("example.com", tt.path, tt.headers)

			switch {
			case route == nil && tt.want == "":
				// ok
			case route == nil && tt.want != "":
				t.Errorf("route=nil, want %q", tt.want)
			case route != nil && tt.want == "":
				t.Errorf("route=%q, want nil", route.Name)
			case route != nil && route.Name != tt.want:
				t.Errorf("route=%q, want %q", route.Name, tt.want)
			}
		})
	}
}

func TestHeadersMatch(t *testing.T) {
	matchers := []httpv1beta1.HeaderMatch{
		{Name: "X-Required-Header", Value: ptr.To("expected")},
		{Name: "X-Key-Only-Header"},
	}

	tests := []struct {
		name    string
		headers map[string][]string
		want    bool
	}{
		{
			name:    "matches exact header",
			headers: map[string][]string{"X-Required-Header": {"expected"}, "X-Key-Only-Header": {"any-value"}},
			want:    true,
		},
		{
			name:    "missing required but has key-only header",
			headers: map[string][]string{"X-Key-Only-Header": {"any-value"}},
			want:    false,
		},
		{
			name:    "missing key-only header",
			headers: map[string][]string{"X-Required-Header": {"expected"}},
			want:    false,
		},
		{
			name:    "wrong value for required header",
			headers: map[string][]string{"X-Required-Header": {"wrong"}, "X-Key-Only-Header": {"any-value"}},
			want:    false,
		},
		{
			name:    "empty string as value",
			headers: map[string][]string{"X-Required-Header": {"expected"}, "X-Key-Only-Header": {""}},
			want:    true,
		},
		{
			name:    "no headers provided",
			headers: map[string][]string{},
			want:    false,
		},
		{
			name:    "extra headers provided",
			headers: map[string][]string{"X-Required-Header": {"expected"}, "X-Key-Only-Header": {"any-value"}, "X-Extra-Header": {"extra"}},
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := headersMatch(matchers, tt.headers)
			if result != tt.want {
				t.Errorf("headersMatch()=%v, want=%v", result, tt.want)
			}
		})
	}

	emptyValueMatchers := []httpv1beta1.HeaderMatch{
		{Name: "X-Empty-Value-Header", Value: ptr.To("")},
	}

	t.Run("matches empty string header value", func(t *testing.T) {
		result := headersMatch(emptyValueMatchers, map[string][]string{"X-Empty-Value-Header": {""}})
		if !result {
			t.Error("expected headers to match")
		}
	})

	t.Run("does not match non-empty string for empty value header", func(t *testing.T) {
		result := headersMatch(emptyValueMatchers, map[string][]string{"X-Empty-Value-Header": {"non-empty"}})
		if result {
			t.Error("expected headers not to match")
		}
	})
}

func TestRoute(t *testing.T) {
	exampleIR := &httpv1beta1.InterceptorRoute{
		ObjectMeta: metav1.ObjectMeta{Name: "test"},
		Spec: httpv1beta1.InterceptorRouteSpec{
			Rules: []httpv1beta1.RoutingRule{{Hosts: []string{"example.com"}}},
		},
	}
	apiIR := &httpv1beta1.InterceptorRoute{
		ObjectMeta: metav1.ObjectMeta{Name: "api"},
		Spec: httpv1beta1.InterceptorRouteSpec{
			Rules: []httpv1beta1.RoutingRule{{
				Hosts: []string{"example.com"},
				Paths: []httpv1beta1.PathMatch{{Value: "/api/"}},
			}},
		},
	}
	rootIR := &httpv1beta1.InterceptorRoute{
		ObjectMeta: metav1.ObjectMeta{Name: "root"},
		Spec: httpv1beta1.InterceptorRouteSpec{
			Rules: []httpv1beta1.RoutingRule{{
				Hosts: []string{"example.com"},
				Paths: []httpv1beta1.PathMatch{{Value: "/"}},
			}},
		},
	}

	tests := []struct {
		name   string
		stored []*httpv1beta1.InterceptorRoute
		host   string
		path   string
		want   string // expected Name, or "" for nil
	}{
		{
			name:   "no matching host returns nil",
			stored: []*httpv1beta1.InterceptorRoute{exampleIR},
			host:   "other.com",
			want:   "",
		},
		{
			name:   "exact host match",
			stored: []*httpv1beta1.InterceptorRoute{exampleIR},
			host:   "example.com",
			path:   "/any/path",
			want:   "test",
		},
		{
			name:   "path prefix match",
			stored: []*httpv1beta1.InterceptorRoute{apiIR},
			host:   "example.com",
			path:   "/api/v1/users",
			want:   "api",
		},
		{
			name:   "path prefix no match",
			stored: []*httpv1beta1.InterceptorRoute{apiIR},
			host:   "example.com",
			path:   "/other/path",
			want:   "",
		},
		{
			name:   "longest path prefix wins",
			stored: []*httpv1beta1.InterceptorRoute{rootIR, apiIR},
			host:   "example.com",
			path:   "/api/v1",
			want:   "api",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tm := NewTableMemory()
			for _, ir := range tt.stored {
				tm = tm.Remember(ir)
			}

			route := tm.Route(tt.host, tt.path, nil)

			switch {
			case route == nil && tt.want == "":
				// ok
			case route == nil && tt.want != "":
				t.Errorf("route=nil, want %q", tt.want)
			case route != nil && tt.want == "":
				t.Errorf("route=%q, want nil", route.Name)
			case route != nil && route.Name != tt.want:
				t.Errorf("route=%q, want %q", route.Name, tt.want)
			}
		})
	}
}

func TestRouteWildcardMultiLevel(t *testing.T) {
	wildcardIR := &httpv1beta1.InterceptorRoute{
		ObjectMeta: metav1.ObjectMeta{Name: "wildcard-example"},
		Spec: httpv1beta1.InterceptorRouteSpec{
			Rules: []httpv1beta1.RoutingRule{{Hosts: []string{"*.example.com"}}},
		},
	}

	t.Run("matches single-level subdomain", func(t *testing.T) {
		runRouteTest(t, []*httpv1beta1.InterceptorRoute{wildcardIR},
			"bar.example.com", "", wildcardIR)
	})

	t.Run("matches nested subdomain", func(t *testing.T) {
		runRouteTest(t, []*httpv1beta1.InterceptorRoute{wildcardIR},
			"foo.bar.example.com", "", wildcardIR)
	})

	t.Run("matches deeply nested subdomain", func(t *testing.T) {
		runRouteTest(t, []*httpv1beta1.InterceptorRoute{wildcardIR},
			"a.b.c.example.com", "", wildcardIR)
	})

	t.Run("rejects different domain", func(t *testing.T) {
		runRouteTest(t, []*httpv1beta1.InterceptorRoute{wildcardIR},
			"foo.other.com", "", nil)
	})
}

func TestRouteWildcardPrecedence(t *testing.T) {
	wildcardIR := &httpv1beta1.InterceptorRoute{
		ObjectMeta: metav1.ObjectMeta{Name: "wildcard-example"},
		Spec: httpv1beta1.InterceptorRouteSpec{
			Rules: []httpv1beta1.RoutingRule{{Hosts: []string{"*.example.com"}}},
		},
	}
	moreSpecificWildcardIR := &httpv1beta1.InterceptorRoute{
		ObjectMeta: metav1.ObjectMeta{Name: "wildcard-bar-example"},
		Spec: httpv1beta1.InterceptorRouteSpec{
			Rules: []httpv1beta1.RoutingRule{{Hosts: []string{"*.bar.example.com"}}},
		},
	}
	exactIR := &httpv1beta1.InterceptorRoute{
		ObjectMeta: metav1.ObjectMeta{Name: "exact-foo"},
		Spec: httpv1beta1.InterceptorRouteSpec{
			Rules: []httpv1beta1.RoutingRule{{Hosts: []string{"foo.example.com"}}},
		},
	}
	catchAllIR := &httpv1beta1.InterceptorRoute{
		ObjectMeta: metav1.ObjectMeta{Name: "catch-all"},
		Spec: httpv1beta1.InterceptorRouteSpec{
			Rules: []httpv1beta1.RoutingRule{{Hosts: []string{"*"}}},
		},
	}

	t.Run("exact wins over wildcard", func(t *testing.T) {
		runRouteTest(t, []*httpv1beta1.InterceptorRoute{wildcardIR, exactIR},
			"foo.example.com", "", exactIR)
	})

	t.Run("exact wins regardless of storage order", func(t *testing.T) {
		runRouteTest(t, []*httpv1beta1.InterceptorRoute{exactIR, wildcardIR},
			"foo.example.com", "", exactIR)
	})

	t.Run("more specific wildcard wins", func(t *testing.T) {
		runRouteTest(t, []*httpv1beta1.InterceptorRoute{wildcardIR, moreSpecificWildcardIR},
			"foo.bar.example.com", "", moreSpecificWildcardIR)
	})

	t.Run("falls back to less specific wildcard", func(t *testing.T) {
		runRouteTest(t, []*httpv1beta1.InterceptorRoute{wildcardIR, moreSpecificWildcardIR},
			"foo.baz.example.com", "", wildcardIR)
	})

	t.Run("wildcard wins over catch-all", func(t *testing.T) {
		runRouteTest(t, []*httpv1beta1.InterceptorRoute{catchAllIR, wildcardIR},
			"bar.example.com", "", wildcardIR)
	})

	t.Run("falls back to catch-all when no wildcard matches", func(t *testing.T) {
		runRouteTest(t, []*httpv1beta1.InterceptorRoute{catchAllIR, wildcardIR},
			"bar.other.com", "", catchAllIR)
	})
}

func TestRouteCatchAll(t *testing.T) {
	catchAllIR := &httpv1beta1.InterceptorRoute{
		ObjectMeta: metav1.ObjectMeta{Name: "catch-all"},
		Spec: httpv1beta1.InterceptorRouteSpec{
			Rules: []httpv1beta1.RoutingRule{{Hosts: []string{"*"}}},
		},
	}
	emptyHostIR := &httpv1beta1.InterceptorRoute{
		ObjectMeta: metav1.ObjectMeta{Name: "empty-host"},
		Spec: httpv1beta1.InterceptorRouteSpec{
			Rules: []httpv1beta1.RoutingRule{{Hosts: []string{""}}},
		},
	}
	nilHostIR := &httpv1beta1.InterceptorRoute{
		ObjectMeta: metav1.ObjectMeta{Name: "nil-host"},
		Spec: httpv1beta1.InterceptorRouteSpec{
			Rules: []httpv1beta1.RoutingRule{{}},
		},
	}

	t.Run("star matches single-label host", func(t *testing.T) {
		runRouteTest(t, []*httpv1beta1.InterceptorRoute{catchAllIR},
			"localhost", "", catchAllIR)
	})

	t.Run("star matches multi-label host", func(t *testing.T) {
		runRouteTest(t, []*httpv1beta1.InterceptorRoute{catchAllIR},
			"foo.example.com", "", catchAllIR)
	})

	t.Run("empty host matches any hostname", func(t *testing.T) {
		runRouteTest(t, []*httpv1beta1.InterceptorRoute{emptyHostIR},
			"anything.example.com", "", emptyHostIR)
	})

	t.Run("nil hosts matches any hostname", func(t *testing.T) {
		runRouteTest(t, []*httpv1beta1.InterceptorRoute{nilHostIR},
			"example.com", "", nilHostIR)
	})
}

func TestRouteWildcardWithPath(t *testing.T) {
	wildcardWithPathIR := &httpv1beta1.InterceptorRoute{
		ObjectMeta: metav1.ObjectMeta{Name: "wildcard-with-path"},
		Spec: httpv1beta1.InterceptorRouteSpec{
			Rules: []httpv1beta1.RoutingRule{{
				Hosts: []string{"*.example.com"},
				Paths: []httpv1beta1.PathMatch{{Value: "/api/"}},
			}},
		},
	}

	t.Run("matches with path prefix", func(t *testing.T) {
		runRouteTest(t, []*httpv1beta1.InterceptorRoute{wildcardWithPathIR},
			"bar.example.com", "/api/v1/users", wildcardWithPathIR)
	})

	t.Run("rejects wrong path", func(t *testing.T) {
		runRouteTest(t, []*httpv1beta1.InterceptorRoute{wildcardWithPathIR},
			"bar.example.com", "/other/", nil)
	})
}

func TestRouteMultipleRules(t *testing.T) {
	multiRuleIR := &httpv1beta1.InterceptorRoute{
		ObjectMeta: metav1.ObjectMeta{Name: "multi-rule"},
		Spec: httpv1beta1.InterceptorRouteSpec{
			Rules: []httpv1beta1.RoutingRule{
				{Hosts: []string{"alpha.example.com"}, Paths: []httpv1beta1.PathMatch{{Value: "/api/"}}},
				{Hosts: []string{"beta.example.com"}},
			},
		},
	}

	tm := NewTableMemory().Remember(multiRuleIR)

	tests := map[string]struct {
		host      string
		path      string
		wantMatch bool
	}{
		"matches first rule by host and path": {
			host:      "alpha.example.com",
			path:      "/api/v1/resource",
			wantMatch: true,
		},
		"first rule rejects wrong path": {
			host:      "alpha.example.com",
			path:      "/other/",
			wantMatch: false,
		},
		"matches second rule by host (any path)": {
			host:      "beta.example.com",
			path:      "/anything",
			wantMatch: true,
		},
		"no match for unknown host": {
			host:      "gamma.example.com",
			path:      "/api/v1/resource",
			wantMatch: false,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			route := tm.Route(tt.host, tt.path, nil)
			gotMatch := route != nil
			if gotMatch != tt.wantMatch {
				t.Errorf("got %v, want %v", gotMatch, tt.wantMatch)
			}
		})
	}
}

func TestRouteMultipleRulesWithHeaders(t *testing.T) {
	ir := &httpv1beta1.InterceptorRoute{
		ObjectMeta: metav1.ObjectMeta{Name: "multi-rule-headers"},
		Spec: httpv1beta1.InterceptorRouteSpec{
			Rules: []httpv1beta1.RoutingRule{
				{
					Hosts:   []string{"example.com"},
					Paths:   []httpv1beta1.PathMatch{{Value: "/api/"}},
					Headers: []httpv1beta1.HeaderMatch{{Name: "X-Version", Value: ptr.To("v1")}},
				},
				{
					Hosts:   []string{"example.com"},
					Paths:   []httpv1beta1.PathMatch{{Value: "/web/"}},
					Headers: []httpv1beta1.HeaderMatch{{Name: "X-Mode", Value: ptr.To("debug")}},
				},
			},
		},
	}

	tm := NewTableMemory().Remember(ir)

	tests := map[string]struct {
		path      string
		headers   map[string][]string
		wantMatch bool
	}{
		"matches first rule with correct path and header": {
			path:      "/api/v1/resource",
			headers:   map[string][]string{"X-Version": {"v1"}},
			wantMatch: true,
		},
		"first rule rejects wrong header": {
			path:      "/api/v1/resource",
			headers:   map[string][]string{"X-Version": {"v2"}},
			wantMatch: false,
		},
		"matches second rule with correct path and header": {
			path:      "/web/dashboard",
			headers:   map[string][]string{"X-Mode": {"debug"}},
			wantMatch: true,
		},
		"second rule rejects wrong header": {
			path:      "/web/dashboard",
			headers:   map[string][]string{"X-Mode": {"prod"}},
			wantMatch: false,
		},
		"no match for path from rule 1 with header from rule 2": {
			path:      "/api/v1/resource",
			headers:   map[string][]string{"X-Mode": {"debug"}},
			wantMatch: false,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			route := tm.Route("example.com", tt.path, tt.headers)
			gotMatch := route != nil
			if gotMatch != tt.wantMatch {
				t.Errorf("got %v, want %v", gotMatch, tt.wantMatch)
			}
		})
	}
}

func TestRouteMultipleRulesWithWildcards(t *testing.T) {
	ir := &httpv1beta1.InterceptorRoute{
		ObjectMeta: metav1.ObjectMeta{Name: "multi-rule-wildcard"},
		Spec: httpv1beta1.InterceptorRouteSpec{
			Rules: []httpv1beta1.RoutingRule{
				{Hosts: []string{"exact.example.com"}},
				{Hosts: []string{"*.wildcard.com"}},
			},
		},
	}

	tm := NewTableMemory().Remember(ir)

	tests := map[string]struct {
		host      string
		wantMatch bool
	}{
		"matches exact host from first rule": {
			host:      "exact.example.com",
			wantMatch: true,
		},
		"matches wildcard from second rule": {
			host:      "foo.wildcard.com",
			wantMatch: true,
		},
		"matches nested wildcard from second rule": {
			host:      "bar.foo.wildcard.com",
			wantMatch: true,
		},
		"no match for unrelated host": {
			host:      "other.example.com",
			wantMatch: false,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			route := tm.Route(tt.host, "", nil)
			gotMatch := route != nil
			if gotMatch != tt.wantMatch {
				t.Errorf("got %v, want %v", gotMatch, tt.wantMatch)
			}
		})
	}
}

// runRouteTest is a helper that creates a TableMemory, stores the given
// InterceptorRoutes, and verifies that Route returns the expected result.
func runRouteTest(t *testing.T, stored []*httpv1beta1.InterceptorRoute, reqHost, reqPath string, want *httpv1beta1.InterceptorRoute) {
	t.Helper()
	tm := NewTableMemory()
	for _, ir := range stored {
		tm = tm.Remember(ir)
	}

	route := tm.Route(reqHost, reqPath, nil)

	switch {
	case route == nil && want == nil:
		// ok
	case route == nil:
		t.Errorf("route=nil, want=%q", want.Name)
	case want == nil:
		t.Errorf("route=%q, want=nil", route.Name)
	case route.Name != want.Name:
		t.Errorf("route=%q, want=%q", route.Name, want.Name)
	}
}
