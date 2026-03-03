package middleware

import (
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	httpv1alpha1 "github.com/kedacore/http-add-on/operator/apis/http/v1alpha1"
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

			rm := NewRouting(routingTable, upstreamHandler, fakeClient, false)
			Expect(rm).NotTo(BeNil())
			Expect(rm.routingTable).To(Equal(routingTable))
			Expect(rm.upstreamHandler).To(Equal(upstreamHandler))
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

			httpso = httpv1alpha1.HTTPScaledObject{
				Spec: httpv1alpha1.HTTPScaledObjectSpec{
					Hosts: []string{
						host,
					},
					ScaleTargetRef: httpv1alpha1.ScaleTargetRef{
						Port: 80,
					},
				},
			}

			httpsoWithPortName = httpv1alpha1.HTTPScaledObject{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "keda",
					Namespace: "default",
				},
				Spec: httpv1alpha1.HTTPScaledObjectSpec{
					Hosts: []string{
						"keda2.sh",
					},
					ScaleTargetRef: httpv1alpha1.ScaleTargetRef{
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
			routingMiddleware = NewRouting(routingTable, upstreamHandler, client, false)

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

				routingTable.Memory[host] = &httpso

				routingMiddleware.ServeHTTP(w, r)
				Expect(uh).To(BeTrue())
				Expect(w.Code).To(Equal(sc))
				Expect(w.Body.String()).To(Equal(st))
			})
		})

		When("route is found with portName", func() {
			It("routes to the upstream handler", func() {
				clientWithSvc := fake.NewClientBuilder().WithScheme(cache.NewScheme()).WithObjects(svc).Build()
				routingMiddleware = NewRouting(routingTable, upstreamHandler, clientWithSvc, false)

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

				routingTable.Memory["keda2.sh"] = &httpsoWithPortName

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

				routingTable.Memory["keda2.sh"] = &httpsoWithPortName

				r.Host = "keda2.sh"
				routingMiddleware.ServeHTTP(w, r)
				Expect(uh).To(BeFalse())
				Expect(w.Code).To(Equal(http.StatusInternalServerError))
				Expect(w.Body.String()).To(Equal("Internal Server Error"))
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
				Expect(w.Body.String()).To(Equal(st))
			})
		})
	})
})
