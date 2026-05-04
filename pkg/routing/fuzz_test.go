package routing

import (
	"net/http"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	httpv1beta1 "github.com/kedacore/http-add-on/operator/apis/http/v1beta1"
)

func FuzzNewKey(f *testing.F) {
	f.Add("example.com", "/api/v1")
	f.Add("", "")
	f.Add("k8s.io", "//abc/def//")
	f.Add("[::1]:8080", "/path")
	f.Add("host\x00name", "/path\x00with\x00nulls")
	f.Add("a.b.c.d.e.f.g.h.i.j.example.com", "/"+string(make([]byte, 1024)))

	f.Fuzz(func(t *testing.T, hostname, path string) {
		key := NewKey(hostname, path)
		s := key.String()
		if len(s) == 0 || s[len(hostname)] != '/' {
			t.Errorf("NewKey(%q, %q) = %q, missing '/' separator after hostname", hostname, path, s)
		}
	})
}

func FuzzStripPort(f *testing.F) {
	f.Add("example.com")
	f.Add("example.com:8080")
	f.Add("[::1]:8080")
	f.Add("")
	f.Add(":")
	f.Add(":8080")
	f.Add("[::1]")
	f.Add("host:port:extra")

	f.Fuzz(func(t *testing.T, host string) {
		_ = stripPort(host)
	})
}

func FuzzWildcardHostnames(f *testing.F) {
	f.Add("a.b.example.com")
	f.Add("localhost")
	f.Add("")
	f.Add("a.b.c.d.e.f.g.h.i.j.k")
	f.Add(".....")
	f.Add(".leading.dot")
	f.Add("trailing.dot.")

	f.Fuzz(func(t *testing.T, hostname string) {
		result := wildcardHostnames(hostname)
		for _, w := range result {
			if len(w) < 2 || w[0] != '*' || w[1] != '.' {
				t.Errorf("wildcard %q does not start with '*.'", w)
			}
		}
	})
}

// fuzzTableMemory builds a pre-populated TableMemory for fuzz tests.
func fuzzTableMemory() *TableMemory {
	routes := []*httpv1beta1.InterceptorRoute{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "exact-host"},
			Spec: httpv1beta1.InterceptorRouteSpec{
				Rules: []httpv1beta1.RoutingRule{{
					Hosts: []string{"example.com"},
					Paths: []httpv1beta1.PathMatch{{Value: "/api/"}},
				}},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "wildcard-host"},
			Spec: httpv1beta1.InterceptorRouteSpec{
				Rules: []httpv1beta1.RoutingRule{{
					Hosts: []string{"*.example.com"},
				}},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "catch-all"},
			Spec: httpv1beta1.InterceptorRouteSpec{
				Rules: []httpv1beta1.RoutingRule{{
					Hosts: []string{"*"},
					Paths: []httpv1beta1.PathMatch{{Value: "/health"}},
				}},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "with-headers"},
			Spec: httpv1beta1.InterceptorRouteSpec{
				Rules: []httpv1beta1.RoutingRule{{
					Hosts:   []string{"headers.example.com"},
					Headers: []httpv1beta1.HeaderMatch{{Name: "X-Route", Value: ptr.To("v1")}},
				}},
			},
		},
	}

	tm := NewTableMemory()
	for _, ir := range routes {
		tm = tm.Remember(ir)
	}
	return tm
}

var knownRouteNames = map[string]bool{
	"exact-host":    true,
	"wildcard-host": true,
	"catch-all":     true,
	"with-headers":  true,
}

func FuzzTableMemoryRoute(f *testing.F) {
	f.Add("example.com", "/api/v1", "X-Route", "v1")
	f.Add("sub.example.com", "/", "", "")
	f.Add("unknown.host", "/path", "", "")
	f.Add("headers.example.com", "/", "X-Route", "v1")
	f.Add("", "", "", "")
	f.Add("[::1]:8080", "/api/../secret", "X-Forwarded-For", "127.0.0.1")
	f.Add("example.com", "/health", "", "")

	tm := fuzzTableMemory()

	f.Fuzz(func(t *testing.T, hostname, path, headerKey, headerVal string) {
		var headers http.Header
		if headerKey != "" {
			headers = http.Header{headerKey: {headerVal}}
		}

		ir := tm.Route(hostname, path, headers)
		if ir != nil && !knownRouteNames[ir.Name] {
			t.Errorf("Route returned unknown InterceptorRoute name %q", ir.Name)
		}
	})
}
