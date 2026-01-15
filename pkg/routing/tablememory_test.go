package routing

import (
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	httpv1alpha1 "github.com/kedacore/http-add-on/operator/apis/http/v1alpha1"
)

func TestRemember(t *testing.T) {
	t.Run("stores and routes single object", func(t *testing.T) {
		httpso := &httpv1alpha1.HTTPScaledObject{
			ObjectMeta: metav1.ObjectMeta{Name: "test"},
			Spec:       httpv1alpha1.HTTPScaledObjectSpec{Hosts: []string{"example.com"}},
		}

		tm := NewTableMemory().Remember(httpso)

		route := tm.Route("example.com", "")
		if route == nil {
			t.Fatal("no route matched")
		}
		if route.Name != "test" {
			t.Errorf("name=%q, want=%q", route.Name, "test")
		}
	})

	t.Run("retains other objects", func(t *testing.T) {
		httpso1 := &httpv1alpha1.HTTPScaledObject{
			ObjectMeta: metav1.ObjectMeta{Name: "first"},
			Spec:       httpv1alpha1.HTTPScaledObjectSpec{Hosts: []string{"first.com"}},
		}
		httpso2 := &httpv1alpha1.HTTPScaledObject{
			ObjectMeta: metav1.ObjectMeta{Name: "second"},
			Spec:       httpv1alpha1.HTTPScaledObjectSpec{Hosts: []string{"second.com"}},
		}

		tm := NewTableMemory().Remember(httpso1).Remember(httpso2)

		route1 := tm.Route("first.com", "")
		if route1 == nil || route1.Name != "first" {
			name := "<nil>"
			if route1 != nil {
				name = route1.Name
			}
			t.Errorf("name=%q, want=%q", name, "first")
		}
		route2 := tm.Route("second.com", "")
		if route2 == nil || route2.Name != "second" {
			name := "<nil>"
			if route2 != nil {
				name = route2.Name
			}
			t.Errorf("name=%q, want=%q", name, "second")
		}
	})

	t.Run("deep copies input", func(t *testing.T) {
		httpso := &httpv1alpha1.HTTPScaledObject{
			ObjectMeta: metav1.ObjectMeta{Name: "test"},
			Spec:       httpv1alpha1.HTTPScaledObjectSpec{Hosts: []string{"example.com"}},
		}

		tm := NewTableMemory().Remember(httpso)

		// Modify original after storing
		httpso.Spec.Hosts[0] = "modified.com"

		// Should still route to original host, not modified one
		if tm.Route("example.com", "") == nil {
			t.Error("expected route for original host")
		}
		if tm.Route("modified.com", "") != nil {
			t.Error("expected no route for modified host")
		}
	})

	t.Run("replaces object with same host", func(t *testing.T) {
		httpso1 := &httpv1alpha1.HTTPScaledObject{
			ObjectMeta: metav1.ObjectMeta{Name: "v1"},
			Spec:       httpv1alpha1.HTTPScaledObjectSpec{Hosts: []string{"example.com"}},
		}
		httpso2 := &httpv1alpha1.HTTPScaledObject{
			ObjectMeta: metav1.ObjectMeta{Name: "v2"},
			Spec:       httpv1alpha1.HTTPScaledObjectSpec{Hosts: []string{"example.com"}},
		}

		tm := NewTableMemory().Remember(httpso1).Remember(httpso2)

		route := tm.Route("example.com", "")
		if route == nil {
			t.Fatal("no route matched")
		}
		if route.Name != "v2" {
			t.Errorf("name=%q, want=%q", route.Name, "v2")
		}
	})
}

func TestRememberOldestWins(t *testing.T) {
	now := time.Now()

	// Two objects with same host, different creation times
	older := &httpv1alpha1.HTTPScaledObject{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "older",
			CreationTimestamp: metav1.NewTime(now.Add(-time.Hour)),
		},
		Spec: httpv1alpha1.HTTPScaledObjectSpec{Hosts: []string{"example.com"}},
	}
	newer := &httpv1alpha1.HTTPScaledObject{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "newer",
			CreationTimestamp: metav1.NewTime(now),
		},
		Spec: httpv1alpha1.HTTPScaledObjectSpec{Hosts: []string{"example.com"}},
	}

	t.Run("older object wins when added first", func(t *testing.T) {
		tm := NewTableMemory().Remember(older).Remember(newer)

		route := tm.Route("example.com", "")
		if route == nil {
			t.Fatal("no route matched")
		}
		if route.Name != "older" {
			t.Errorf("name=%q, want=%q", route.Name, "older")
		}
	})

	t.Run("older object wins when added second", func(t *testing.T) {
		tm := NewTableMemory().Remember(newer).Remember(older)

		route := tm.Route("example.com", "")
		if route == nil {
			t.Fatal("no route matched")
		}
		if route.Name != "older" {
			t.Errorf("name=%q, want=%q", route.Name, "older")
		}
	})
}

func TestRoute(t *testing.T) {
	exampleHTTPSO := &httpv1alpha1.HTTPScaledObject{
		ObjectMeta: metav1.ObjectMeta{Name: "test"},
		Spec:       httpv1alpha1.HTTPScaledObjectSpec{Hosts: []string{"example.com"}},
	}
	apiHTTPSO := &httpv1alpha1.HTTPScaledObject{
		ObjectMeta: metav1.ObjectMeta{Name: "api"},
		Spec:       httpv1alpha1.HTTPScaledObjectSpec{Hosts: []string{"example.com"}, PathPrefixes: []string{"/api/"}},
	}
	rootHTTPSO := &httpv1alpha1.HTTPScaledObject{
		ObjectMeta: metav1.ObjectMeta{Name: "root"},
		Spec:       httpv1alpha1.HTTPScaledObjectSpec{Hosts: []string{"example.com"}, PathPrefixes: []string{"/"}},
	}

	tests := []struct {
		name   string
		stored []*httpv1alpha1.HTTPScaledObject
		host   string
		path   string
		want   string // expected Name, or "" for nil
	}{
		{
			name:   "no matching host returns nil",
			stored: []*httpv1alpha1.HTTPScaledObject{exampleHTTPSO},
			host:   "other.com",
			want:   "",
		},
		{
			name:   "exact host match",
			stored: []*httpv1alpha1.HTTPScaledObject{exampleHTTPSO},
			host:   "example.com",
			path:   "/any/path",
			want:   "test",
		},
		{
			name:   "path prefix match",
			stored: []*httpv1alpha1.HTTPScaledObject{apiHTTPSO},
			host:   "example.com",
			path:   "/api/v1/users",
			want:   "api",
		},
		{
			name:   "path prefix no match",
			stored: []*httpv1alpha1.HTTPScaledObject{apiHTTPSO},
			host:   "example.com",
			path:   "/other/path",
			want:   "",
		},
		{
			name:   "longest path prefix wins",
			stored: []*httpv1alpha1.HTTPScaledObject{rootHTTPSO, apiHTTPSO},
			host:   "example.com",
			path:   "/api/v1",
			want:   "api",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tm := NewTableMemory()
			for _, httpso := range tt.stored {
				tm = tm.Remember(httpso)
			}

			route := tm.Route(tt.host, tt.path)

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
	wildcardHTTPSO := &httpv1alpha1.HTTPScaledObject{
		ObjectMeta: metav1.ObjectMeta{Name: "wildcard-example"},
		Spec:       httpv1alpha1.HTTPScaledObjectSpec{Hosts: []string{"*.example.com"}},
	}

	t.Run("matches single-level subdomain", func(t *testing.T) {
		runRouteTest(t, []*httpv1alpha1.HTTPScaledObject{wildcardHTTPSO},
			"bar.example.com", "", wildcardHTTPSO)
	})

	t.Run("matches nested subdomain", func(t *testing.T) {
		runRouteTest(t, []*httpv1alpha1.HTTPScaledObject{wildcardHTTPSO},
			"foo.bar.example.com", "", wildcardHTTPSO)
	})

	t.Run("matches deeply nested subdomain", func(t *testing.T) {
		runRouteTest(t, []*httpv1alpha1.HTTPScaledObject{wildcardHTTPSO},
			"a.b.c.example.com", "", wildcardHTTPSO)
	})

	t.Run("rejects different domain", func(t *testing.T) {
		runRouteTest(t, []*httpv1alpha1.HTTPScaledObject{wildcardHTTPSO},
			"foo.other.com", "", nil)
	})
}

func TestRouteWildcardPrecedence(t *testing.T) {
	wildcardHTTPSO := &httpv1alpha1.HTTPScaledObject{
		ObjectMeta: metav1.ObjectMeta{Name: "wildcard-example"},
		Spec:       httpv1alpha1.HTTPScaledObjectSpec{Hosts: []string{"*.example.com"}},
	}
	moreSpecificWildcardHTTPSO := &httpv1alpha1.HTTPScaledObject{
		ObjectMeta: metav1.ObjectMeta{Name: "wildcard-bar-example"},
		Spec:       httpv1alpha1.HTTPScaledObjectSpec{Hosts: []string{"*.bar.example.com"}},
	}
	exactHTTPSO := &httpv1alpha1.HTTPScaledObject{
		ObjectMeta: metav1.ObjectMeta{Name: "exact-foo"},
		Spec:       httpv1alpha1.HTTPScaledObjectSpec{Hosts: []string{"foo.example.com"}},
	}
	catchAllHTTPSO := &httpv1alpha1.HTTPScaledObject{
		ObjectMeta: metav1.ObjectMeta{Name: "catch-all"},
		Spec:       httpv1alpha1.HTTPScaledObjectSpec{Hosts: []string{"*"}},
	}

	t.Run("exact wins over wildcard", func(t *testing.T) {
		runRouteTest(t, []*httpv1alpha1.HTTPScaledObject{wildcardHTTPSO, exactHTTPSO},
			"foo.example.com", "", exactHTTPSO)
	})

	t.Run("exact wins regardless of storage order", func(t *testing.T) {
		runRouteTest(t, []*httpv1alpha1.HTTPScaledObject{exactHTTPSO, wildcardHTTPSO},
			"foo.example.com", "", exactHTTPSO)
	})

	t.Run("more specific wildcard wins", func(t *testing.T) {
		runRouteTest(t, []*httpv1alpha1.HTTPScaledObject{wildcardHTTPSO, moreSpecificWildcardHTTPSO},
			"foo.bar.example.com", "", moreSpecificWildcardHTTPSO)
	})

	t.Run("falls back to less specific wildcard", func(t *testing.T) {
		runRouteTest(t, []*httpv1alpha1.HTTPScaledObject{wildcardHTTPSO, moreSpecificWildcardHTTPSO},
			"foo.baz.example.com", "", wildcardHTTPSO)
	})

	t.Run("wildcard wins over catch-all", func(t *testing.T) {
		runRouteTest(t, []*httpv1alpha1.HTTPScaledObject{catchAllHTTPSO, wildcardHTTPSO},
			"bar.example.com", "", wildcardHTTPSO)
	})

	t.Run("falls back to catch-all when no wildcard matches", func(t *testing.T) {
		runRouteTest(t, []*httpv1alpha1.HTTPScaledObject{catchAllHTTPSO, wildcardHTTPSO},
			"bar.other.com", "", catchAllHTTPSO)
	})
}

func TestRouteCatchAll(t *testing.T) {
	catchAllHTTPSO := &httpv1alpha1.HTTPScaledObject{
		ObjectMeta: metav1.ObjectMeta{Name: "catch-all"},
		Spec:       httpv1alpha1.HTTPScaledObjectSpec{Hosts: []string{"*"}},
	}
	emptyHostHTTPSO := &httpv1alpha1.HTTPScaledObject{
		ObjectMeta: metav1.ObjectMeta{Name: "empty-host"},
		Spec:       httpv1alpha1.HTTPScaledObjectSpec{Hosts: []string{""}},
	}
	nilHostHTTPSO := &httpv1alpha1.HTTPScaledObject{
		ObjectMeta: metav1.ObjectMeta{Name: "nil-host"},
		Spec:       httpv1alpha1.HTTPScaledObjectSpec{Hosts: nil},
	}

	t.Run("star matches single-label host", func(t *testing.T) {
		runRouteTest(t, []*httpv1alpha1.HTTPScaledObject{catchAllHTTPSO},
			"localhost", "", catchAllHTTPSO)
	})

	t.Run("star matches multi-label host", func(t *testing.T) {
		runRouteTest(t, []*httpv1alpha1.HTTPScaledObject{catchAllHTTPSO},
			"foo.example.com", "", catchAllHTTPSO)
	})

	t.Run("empty host matches any hostname", func(t *testing.T) {
		runRouteTest(t, []*httpv1alpha1.HTTPScaledObject{emptyHostHTTPSO},
			"anything.example.com", "", emptyHostHTTPSO)
	})

	t.Run("nil hosts matches any hostname", func(t *testing.T) {
		runRouteTest(t, []*httpv1alpha1.HTTPScaledObject{nilHostHTTPSO},
			"example.com", "", nilHostHTTPSO)
	})
}

func TestRouteWildcardWithPath(t *testing.T) {
	wildcardWithPathHTTPSO := &httpv1alpha1.HTTPScaledObject{
		ObjectMeta: metav1.ObjectMeta{Name: "wildcard-with-path"},
		Spec: httpv1alpha1.HTTPScaledObjectSpec{
			Hosts:        []string{"*.example.com"},
			PathPrefixes: []string{"/api/"},
		},
	}

	t.Run("matches with path prefix", func(t *testing.T) {
		runRouteTest(t, []*httpv1alpha1.HTTPScaledObject{wildcardWithPathHTTPSO},
			"bar.example.com", "/api/v1/users", wildcardWithPathHTTPSO)
	})

	t.Run("rejects wrong path", func(t *testing.T) {
		runRouteTest(t, []*httpv1alpha1.HTTPScaledObject{wildcardWithPathHTTPSO},
			"bar.example.com", "/other/", nil)
	})
}

// runRouteTest is a helper that creates a TableMemory, stores the given
// HTTPScaledObjects, and verifies that Route returns the expected result.
func runRouteTest(t *testing.T, stored []*httpv1alpha1.HTTPScaledObject, reqHost, reqPath string, want *httpv1alpha1.HTTPScaledObject) {
	t.Helper()
	tm := NewTableMemory()
	for _, httpso := range stored {
		tm = tm.Remember(httpso)
	}

	route := tm.Route(reqHost, reqPath)

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
