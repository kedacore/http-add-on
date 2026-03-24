package routing

import (
	"strconv"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	httpv1beta1 "github.com/kedacore/http-add-on/operator/apis/http/v1beta1"
)

func BenchmarkExactMatch(b *testing.B) {
	b.Run("SingleRoute", func(b *testing.B) {
		ir := &httpv1beta1.InterceptorRoute{
			ObjectMeta: metav1.ObjectMeta{Name: "exact"},
			Spec: httpv1beta1.InterceptorRouteSpec{
				Rules: []httpv1beta1.RoutingRule{{Hosts: []string{"foo.example.com"}}},
			},
		}
		tm := NewTableMemory().Remember(ir)

		for b.Loop() {
			tm.Route("foo.example.com", "/api/v1", nil)
		}
	})

	b.Run("Among100Routes", func(b *testing.B) {
		tm := setup100ExactRoutes()

		for b.Loop() {
			tm.Route("host50.example.com", "/api/v1", nil)
		}
	})
}

func BenchmarkWildcard(b *testing.B) {
	b.Run("1Subdomain", func(b *testing.B) {
		ir := &httpv1beta1.InterceptorRoute{
			ObjectMeta: metav1.ObjectMeta{Name: "wildcard"},
			Spec: httpv1beta1.InterceptorRouteSpec{
				Rules: []httpv1beta1.RoutingRule{{Hosts: []string{"*.example.com"}}},
			},
		}
		tm := NewTableMemory().Remember(ir)

		for b.Loop() {
			tm.Route("foo.example.com", "/api/v1", nil)
		}
	})

	b.Run("3Subdomains", func(b *testing.B) {
		ir := &httpv1beta1.InterceptorRoute{
			ObjectMeta: metav1.ObjectMeta{Name: "wildcard"},
			Spec: httpv1beta1.InterceptorRouteSpec{
				Rules: []httpv1beta1.RoutingRule{{Hosts: []string{"*.example.com"}}},
			},
		}
		tm := NewTableMemory().Remember(ir)

		for b.Loop() {
			tm.Route("a.b.c.example.com", "/api/v1", nil)
		}
	})

	b.Run("5Subdomains", func(b *testing.B) {
		ir := &httpv1beta1.InterceptorRoute{
			ObjectMeta: metav1.ObjectMeta{Name: "wildcard"},
			Spec: httpv1beta1.InterceptorRouteSpec{
				Rules: []httpv1beta1.RoutingRule{{Hosts: []string{"*.example.com"}}},
			},
		}
		tm := NewTableMemory().Remember(ir)

		for b.Loop() {
			tm.Route("a.b.c.d.e.example.com", "/api/v1", nil)
		}
	})

	b.Run("Among100Routes", func(b *testing.B) {
		tm := setup100ExactRoutes()
		wildcardIR := &httpv1beta1.InterceptorRoute{
			ObjectMeta: metav1.ObjectMeta{Name: "wildcard"},
			Spec: httpv1beta1.InterceptorRouteSpec{
				Rules: []httpv1beta1.RoutingRule{{Hosts: []string{"*.other.com"}}},
			},
		}
		tm = tm.Remember(wildcardIR)

		for b.Loop() {
			tm.Route("foo.other.com", "/api/v1", nil)
		}
	})
}

func BenchmarkCatchAll(b *testing.B) {
	b.Run("SingleRoute", func(b *testing.B) {
		ir := &httpv1beta1.InterceptorRoute{
			ObjectMeta: metav1.ObjectMeta{Name: "catchall"},
			Spec: httpv1beta1.InterceptorRouteSpec{
				Rules: []httpv1beta1.RoutingRule{{Hosts: []string{"*"}}},
			},
		}
		tm := NewTableMemory().Remember(ir)

		for b.Loop() {
			tm.Route("unknown.domain.com", "/api/v1", nil)
		}
	})

	b.Run("Among100Routes", func(b *testing.B) {
		tm := setup100ExactRoutes()
		catchAllIR := &httpv1beta1.InterceptorRoute{
			ObjectMeta: metav1.ObjectMeta{Name: "catchall"},
			Spec: httpv1beta1.InterceptorRouteSpec{
				Rules: []httpv1beta1.RoutingRule{{Hosts: []string{"*"}}},
			},
		}
		tm = tm.Remember(catchAllIR)

		for b.Loop() {
			tm.Route("unknown.domain.com", "/api/v1", nil)
		}
	})
}

func BenchmarkNoMatch(b *testing.B) {
	b.Run("SingleRoute", func(b *testing.B) {
		ir := &httpv1beta1.InterceptorRoute{
			ObjectMeta: metav1.ObjectMeta{Name: "exact"},
			Spec: httpv1beta1.InterceptorRouteSpec{
				Rules: []httpv1beta1.RoutingRule{{Hosts: []string{"foo.example.com"}}},
			},
		}
		tm := NewTableMemory().Remember(ir)

		for b.Loop() {
			tm.Route("other.domain.com", "/api/v1", nil)
		}
	})

	b.Run("Among100Routes", func(b *testing.B) {
		tm := setup100ExactRoutes()

		for b.Loop() {
			tm.Route("unknown.domain.com", "/api/v1", nil)
		}
	})
}

func setup100ExactRoutes() *TableMemory {
	tm := NewTableMemory()
	for i := range 100 {
		ir := &httpv1beta1.InterceptorRoute{
			ObjectMeta: metav1.ObjectMeta{Name: "exact-" + strconv.Itoa(i)},
			Spec: httpv1beta1.InterceptorRouteSpec{
				Rules: []httpv1beta1.RoutingRule{{Hosts: []string{"host" + strconv.Itoa(i) + ".example.com"}}},
			},
		}
		tm = tm.Remember(ir)
	}
	return tm
}
