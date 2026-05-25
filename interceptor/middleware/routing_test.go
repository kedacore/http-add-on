package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	httpv1beta1 "github.com/kedacore/http-add-on/operator/apis/http/v1beta1"
	"github.com/kedacore/http-add-on/pkg/cache"
	routingtest "github.com/kedacore/http-add-on/pkg/routing/test"
	"github.com/kedacore/http-add-on/pkg/util"
)

const requestTimeout = 60 * time.Second

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

			rm := NewRouting(upstreamHandler, routingTable, fakeClient, false, requestTimeout)
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
			routingMiddleware = NewRouting(upstreamHandler, routingTable, client, false, requestTimeout)

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

		When("route is found with TLS enabled", func() {
			It("stores the upstream hostname as server name in context", func() {
				irTLS := &httpv1beta1.InterceptorRoute{
					ObjectMeta: metav1.ObjectMeta{Namespace: "default"},
					Spec: httpv1beta1.InterceptorRouteSpec{
						Target: httpv1beta1.TargetRef{Service: "my-svc", Port: 8443},
					},
				}
				tlsMiddleware := NewRouting(upstreamHandler, routingTable, client, true, requestTimeout)

				var capturedServerName string
				upstreamHandler.Handle(path, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					capturedServerName = util.UpstreamServerNameFromContext(r.Context())
					w.WriteHeader(http.StatusOK)
				}))

				routingTable.Memory[host] = irTLS
				tlsMiddleware.ServeHTTP(w, r)
				Expect(capturedServerName).To(Equal("my-svc.default"))
			})
		})

		When("route is found with TLS disabled", func() {
			It("stores an empty server name in context", func() {
				var capturedServerName string
				upstreamHandler.Handle(path, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					capturedServerName = util.UpstreamServerNameFromContext(r.Context())
					w.WriteHeader(http.StatusOK)
				}))

				routingTable.Memory[host] = &ir
				routingMiddleware.ServeHTTP(w, r)
				Expect(capturedServerName).To(Equal(""))
			})
		})

		When("route is found with portName", func() {
			It("routes to the upstream handler", func() {
				clientWithSvc := fake.NewClientBuilder().WithScheme(cache.NewScheme()).WithObjects(svc).Build()
				routingMiddleware = NewRouting(upstreamHandler, routingTable, clientWithSvc, false, requestTimeout)

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

		When("route is found with timeouts", func() {
			It("sets a context deadline from global timeout", func() {
				var hasDeadline bool
				upstreamHandler.Handle(path, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					_, hasDeadline = r.Context().Deadline()
					w.WriteHeader(http.StatusOK)
				}))

				routingTable.Memory[host] = &ir
				routingMiddleware.ServeHTTP(w, r)
				Expect(hasDeadline).To(BeTrue())
			})

			It("uses per-route request timeout override instead of global", func() {
				perRouteTimeout := 5 * time.Second
				irWithTimeout := httpv1beta1.InterceptorRoute{
					Spec: httpv1beta1.InterceptorRouteSpec{
						Target: httpv1beta1.TargetRef{Port: 80},
						Timeouts: httpv1beta1.InterceptorRouteTimeouts{
							Request: &metav1.Duration{Duration: perRouteTimeout},
						},
					},
				}

				var capturedDeadline time.Time
				upstreamHandler.Handle(path, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					capturedDeadline, _ = r.Context().Deadline()
					w.WriteHeader(http.StatusOK)
				}))

				routingTable.Memory[host] = &irWithTimeout
				before := time.Now()
				routingMiddleware.ServeHTTP(w, r)

				deadline := capturedDeadline.Sub(before)
				tolerance := 500 * time.Millisecond
				Expect(deadline).To(
					BeNumerically("~", perRouteTimeout, tolerance),
					fmt.Sprintf("deadline %v should be close to per-route timeout %v (±%v), not the global timeout %v", deadline, perRouteTimeout, tolerance, requestTimeout),
				)
			})
		})

		When("route is found with zero request timeout", func() {
			It("does not set a context deadline", func() {
				noTimeoutMiddleware := NewRouting(upstreamHandler, routingTable, client, false, 0)

				var hasDeadline bool
				upstreamHandler.Handle(path, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					_, hasDeadline = r.Context().Deadline()
					w.WriteHeader(http.StatusOK)
				}))

				routingTable.Memory[host] = &ir
				noTimeoutMiddleware.ServeHTTP(w, r)
				Expect(hasDeadline).To(BeFalse())
			})
		})

		When("per-route request timeout is explicitly zero with non-zero global", func() {
			It("disables the context deadline", func() {
				irWithZeroTimeout := httpv1beta1.InterceptorRoute{
					Spec: httpv1beta1.InterceptorRouteSpec{
						Target: httpv1beta1.TargetRef{Port: 80},
						Timeouts: httpv1beta1.InterceptorRouteTimeouts{
							Request: &metav1.Duration{Duration: 0},
						},
					},
				}

				var hasDeadline bool
				upstreamHandler.Handle(path, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					_, hasDeadline = r.Context().Deadline()
					w.WriteHeader(http.StatusOK)
				}))

				routingTable.Memory[host] = &irWithZeroTimeout
				routingMiddleware.ServeHTTP(w, r)
				Expect(hasDeadline).To(BeFalse())
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

			middleware := NewRouting(inner, table, fakeClient, false, 0)

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
