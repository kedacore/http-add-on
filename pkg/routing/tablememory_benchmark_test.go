package routing

import (
	"strconv"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	httpv1alpha1 "github.com/kedacore/http-add-on/operator/apis/http/v1alpha1"
)

func BenchmarkExactMatch(b *testing.B) {
	b.Run("SingleRoute", func(b *testing.B) {
		httpso := &httpv1alpha1.HTTPScaledObject{
			ObjectMeta: metav1.ObjectMeta{Name: "exact"},
			Spec:       httpv1alpha1.HTTPScaledObjectSpec{Hosts: []string{"foo.example.com"}},
		}
		tm := NewTableMemory().Remember(httpso)

		for b.Loop() {
			tm.Route("foo.example.com", "/api/v1")
		}
	})

	b.Run("Among100Routes", func(b *testing.B) {
		tm := setup100ExactRoutes()

		for b.Loop() {
			tm.Route("host50.example.com", "/api/v1")
		}
	})
}

func BenchmarkWildcard(b *testing.B) {
	b.Run("1Subdomain", func(b *testing.B) {
		httpso := &httpv1alpha1.HTTPScaledObject{
			ObjectMeta: metav1.ObjectMeta{Name: "wildcard"},
			Spec:       httpv1alpha1.HTTPScaledObjectSpec{Hosts: []string{"*.example.com"}},
		}
		tm := NewTableMemory().Remember(httpso)

		for b.Loop() {
			tm.Route("foo.example.com", "/api/v1")
		}
	})

	b.Run("3Subdomains", func(b *testing.B) {
		httpso := &httpv1alpha1.HTTPScaledObject{
			ObjectMeta: metav1.ObjectMeta{Name: "wildcard"},
			Spec:       httpv1alpha1.HTTPScaledObjectSpec{Hosts: []string{"*.example.com"}},
		}
		tm := NewTableMemory().Remember(httpso)

		for b.Loop() {
			tm.Route("a.b.c.example.com", "/api/v1")
		}
	})

	b.Run("5Subdomains", func(b *testing.B) {
		httpso := &httpv1alpha1.HTTPScaledObject{
			ObjectMeta: metav1.ObjectMeta{Name: "wildcard"},
			Spec:       httpv1alpha1.HTTPScaledObjectSpec{Hosts: []string{"*.example.com"}},
		}
		tm := NewTableMemory().Remember(httpso)

		for b.Loop() {
			tm.Route("a.b.c.d.e.example.com", "/api/v1")
		}
	})

	b.Run("Among100Routes", func(b *testing.B) {
		tm := setup100ExactRoutes()
		wildcardHTTPSO := &httpv1alpha1.HTTPScaledObject{
			ObjectMeta: metav1.ObjectMeta{Name: "wildcard"},
			Spec:       httpv1alpha1.HTTPScaledObjectSpec{Hosts: []string{"*.other.com"}},
		}
		tm = tm.Remember(wildcardHTTPSO)

		for b.Loop() {
			tm.Route("foo.other.com", "/api/v1")
		}
	})
}

func BenchmarkCatchAll(b *testing.B) {
	b.Run("SingleRoute", func(b *testing.B) {
		httpso := &httpv1alpha1.HTTPScaledObject{
			ObjectMeta: metav1.ObjectMeta{Name: "catchall"},
			Spec:       httpv1alpha1.HTTPScaledObjectSpec{Hosts: []string{"*"}},
		}
		tm := NewTableMemory().Remember(httpso)

		for b.Loop() {
			tm.Route("unknown.domain.com", "/api/v1")
		}
	})

	b.Run("Among100Routes", func(b *testing.B) {
		tm := setup100ExactRoutes()
		catchAllHTTPSO := &httpv1alpha1.HTTPScaledObject{
			ObjectMeta: metav1.ObjectMeta{Name: "catchall"},
			Spec:       httpv1alpha1.HTTPScaledObjectSpec{Hosts: []string{"*"}},
		}
		tm = tm.Remember(catchAllHTTPSO)

		for b.Loop() {
			tm.Route("unknown.domain.com", "/api/v1")
		}
	})
}

func BenchmarkNoMatch(b *testing.B) {
	b.Run("SingleRoute", func(b *testing.B) {
		httpso := &httpv1alpha1.HTTPScaledObject{
			ObjectMeta: metav1.ObjectMeta{Name: "exact"},
			Spec:       httpv1alpha1.HTTPScaledObjectSpec{Hosts: []string{"foo.example.com"}},
		}
		tm := NewTableMemory().Remember(httpso)

		for b.Loop() {
			tm.Route("other.domain.com", "/api/v1")
		}
	})

	b.Run("Among100Routes", func(b *testing.B) {
		tm := setup100ExactRoutes()

		for b.Loop() {
			tm.Route("unknown.domain.com", "/api/v1")
		}
	})
}

func setup100ExactRoutes() TableMemory {
	tm := NewTableMemory()
	for i := range 100 {
		httpso := &httpv1alpha1.HTTPScaledObject{
			ObjectMeta: metav1.ObjectMeta{Name: "exact-" + strconv.Itoa(i)},
			Spec:       httpv1alpha1.HTTPScaledObjectSpec{Hosts: []string{"host" + strconv.Itoa(i) + ".example.com"}},
		}
		tm = tm.Remember(httpso)
	}
	return tm
}
