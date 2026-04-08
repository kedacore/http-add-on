package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	httpv1beta1 "github.com/kedacore/http-add-on/operator/apis/http/v1beta1"
	"github.com/kedacore/http-add-on/pkg/cache"
	routingtest "github.com/kedacore/http-add-on/pkg/routing/test"
)

var _ = Describe("RoutingMiddleware", func() {
	Context("New", func() {
		It("returns new object with expected fields", func() {
			var (
				routingTable    = routingtest.NewTable()
				upstreamHandler = http.NewServeMux()
			)
			emptyHandler := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
			upstreamHandler.Handle("/upstream", emptyHandler)
			fakeClient := fake.NewClientBuilder().WithScheme(cache.NewScheme()).Build()

			rm := NewRouting(upstreamHandler, routingTable, fakeClient, false)
			Expect(rm).NotTo(BeNil())
			Expect(rm.routingTable).To(Equal(routingTable))
			Expect(rm.next).To(Equal(upstreamHandler))
		})
	})

	Context("ServeHTTP", func() {
		const (
			host = "keda.sh"
			path = "/README"
		)

		var (
			upstreamHandler   *http.ServeMux
			client            client.Reader
			routingTable      *routingtest.Table
			routingMiddleware *Routing
			w                 *httptest.ResponseRecorder
			r                 *http.Request

			ir = httpv1beta1.InterceptorRoute{
				Spec: httpv1beta1.InterceptorRouteSpec{
					Target: httpv1beta1.TargetRef{
						Port: 80,
					},
				},
			}

			irWithPortName = httpv1beta1.InterceptorRoute{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "keda",
					Namespace: "default",
				},
				Spec: httpv1beta1.InterceptorRouteSpec{
					Target: httpv1beta1.TargetRef{
						Service:  "keda-svc",
						PortName: "http",
					},
				},
			}
			svc = &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "keda-svc",
					Namespace: "default",
				},
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{
							Name: "http",
							Port: 80,
						},
					},
				},
			}
		)

		BeforeEach(func() {
			upstreamHandler = http.NewServeMux()
			routingTable = routingtest.NewTable()
			client = fake.NewClientBuilder().WithScheme(cache.NewScheme()).Build()
			routingMiddleware = NewRouting(upstreamHandler, routingTable, client, false)

			w = httptest.NewRecorder()

			r = httptest.NewRequest(http.MethodGet, path, nil)
			r.Host = host
		})

		When("route is found", func() {
			It("routes to the upstream handler", func() {
				var (
					sc = http.StatusTeapot
					st = http.StatusText(sc)
				)

				var uh bool
				upstreamHandler.Handle(path, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusTeapot)

					_, err := w.Write([]byte(st))
					Expect(err).NotTo(HaveOccurred())

					uh = true
				}))

				routingTable.Memory[host] = &ir

				routingMiddleware.ServeHTTP(w, r)
				Expect(uh).To(BeTrue())
				Expect(w.Code).To(Equal(sc))
				Expect(w.Body.String()).To(Equal(st))
			})
		})

		When("route is found with portName", func() {
			It("routes to the upstream handler", func() {
				clientWithSvc := fake.NewClientBuilder().WithScheme(cache.NewScheme()).WithObjects(svc).Build()
				routingMiddleware = NewRouting(upstreamHandler, routingTable, clientWithSvc, false)

				var (
					sc = http.StatusTeapot
					st = http.StatusText(sc)
				)

				var uh bool
				upstreamHandler.Handle(path, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusTeapot)

					_, err := w.Write([]byte(st))
					Expect(err).NotTo(HaveOccurred())

					uh = true
				}))

				routingTable.Memory["keda2.sh"] = &irWithPortName

				r.Host = "keda2.sh"
				routingMiddleware.ServeHTTP(w, r)
				Expect(uh).To(BeTrue())
				Expect(w.Code).To(Equal(sc))
				Expect(w.Body.String()).To(Equal(st))
			})
		})

		When("route is found with portName but endpoints are mismatched", func() {
			It("errors to route to upstream handler", func() {
				var (
					sc = http.StatusTeapot
					st = http.StatusText(sc)
				)

				var uh bool
				upstreamHandler.Handle(path, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusTeapot)

					_, err := w.Write([]byte(st))
					Expect(err).NotTo(HaveOccurred())

					uh = true
				}))

				routingTable.Memory["keda2.sh"] = &irWithPortName

				r.Host = "keda2.sh"
				routingMiddleware.ServeHTTP(w, r)
				Expect(uh).To(BeFalse())
				Expect(w.Code).To(Equal(http.StatusInternalServerError))
				Expect(w.Body.String()).To(Equal("Internal Server Error\n"))
			})
		})

		When("route is not found", func() {
			It("serves 404", func() {
				var (
					sc = http.StatusNotFound
					st = http.StatusText(sc)
				)

				var uh bool
				upstreamHandler.Handle(path, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					uh = true
				}))

				routingMiddleware.ServeHTTP(w, r)

				Expect(uh).To(BeFalse())
				Expect(w.Code).To(Equal(sc))
				Expect(w.Body.String()).To(Equal(st + "\n"))
			})
		})
	})
})

func TestRouting_PopulatesRouteInfo(t *testing.T) {
	tests := map[string]struct {
		ir            *httpv1beta1.InterceptorRoute
		wantName      string
		wantNamespace string
		wantStatus    int
	}{
		"matched route sets name and namespace": {
			ir: &httpv1beta1.InterceptorRoute{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-route",
					Namespace: "my-ns",
				},
				Spec: httpv1beta1.InterceptorRouteSpec{
					Target: httpv1beta1.TargetRef{
						Service: "test-svc",
						Port:    8080,
					},
				},
			},
			wantName:      "my-route",
			wantNamespace: "my-ns",
			wantStatus:    http.StatusOK,
		},
		"unmatched route leaves route info empty": {
			ir:            nil,
			wantName:      "",
			wantNamespace: "",
			wantStatus:    http.StatusNotFound,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			table := routingtest.NewTable()
			fakeClient := fake.NewClientBuilder().WithScheme(cache.NewScheme()).Build()

			inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			host := "test.example.com"
			if tc.ir != nil {
				table.Memory[host] = tc.ir
			}

			middleware := NewRouting(inner, table, fakeClient, false)

			info := &routeInfo{}
			req := httptest.NewRequest("GET", "/path", nil)
			req.Host = host
			req = req.WithContext(contextWithRouteInfo(req.Context(), info))

			rec := httptest.NewRecorder()
			middleware.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status: got %d, want %d", rec.Code, tc.wantStatus)
			}
			if info.Name != tc.wantName {
				t.Fatalf("routeInfo.Name: got %q, want %q", info.Name, tc.wantName)
			}
			if info.Namespace != tc.wantNamespace {
				t.Fatalf("routeInfo.Namespace: got %q, want %q", info.Namespace, tc.wantNamespace)
			}
		})
	}
}
