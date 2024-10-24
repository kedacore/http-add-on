package middleware

import (
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	httpv1alpha1 "github.com/kedacore/http-add-on/operator/apis/http/v1alpha1"
	"github.com/kedacore/http-add-on/pkg/k8s"
	routingtest "github.com/kedacore/http-add-on/pkg/routing/test"
)

var _ = Describe("RoutingMiddleware", func() {
	Context("New", func() {
		It("returns new object with expected fields", func() {
			var (
				routingTable    = routingtest.NewTable()
				probeHandler    = http.NewServeMux()
				upstreamHandler = http.NewServeMux()
			)
			emptyHandler := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
			probeHandler.Handle("/probe", emptyHandler)
			upstreamHandler.Handle("/upstream", emptyHandler)
			svcCache := k8s.NewFakeServiceCache()

			rm := NewRouting(routingTable, probeHandler, upstreamHandler, svcCache, false)
			Expect(rm).NotTo(BeNil())
			Expect(rm.routingTable).To(Equal(routingTable))
			Expect(rm.probeHandler).To(Equal(probeHandler))
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
			probeHandler      *http.ServeMux
			svcCache          *k8s.FakeServiceCache
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
			probeHandler = http.NewServeMux()
			routingTable = routingtest.NewTable()
			svcCache = k8s.NewFakeServiceCache()
			routingMiddleware = NewRouting(routingTable, probeHandler, upstreamHandler, svcCache, false)

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

				var ph bool
				probeHandler.Handle(path, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					ph = true
				}))

				routingTable.Memory[host] = &httpso

				routingMiddleware.ServeHTTP(w, r)
				Expect(uh).To(BeTrue())
				Expect(ph).To(BeFalse())
				Expect(w.Code).To(Equal(sc))
				Expect(w.Body.String()).To(Equal(st))
			})
		})

		When("route is found with portName", func() {
			It("routes to the upstream handler", func() {
				svcCache.Add(*svc)
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

				var ph bool
				probeHandler.Handle(path, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					ph = true
				}))

				routingTable.Memory["keda2.sh"] = &httpsoWithPortName

				r.Host = "keda2.sh"
				routingMiddleware.ServeHTTP(w, r)
				Expect(uh).To(BeTrue())
				Expect(ph).To(BeFalse())
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

				var ph bool
				probeHandler.Handle(path, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					ph = true
				}))

				routingTable.Memory["keda2.sh"] = &httpsoWithPortName

				r.Host = "keda2.sh"
				routingMiddleware.ServeHTTP(w, r)
				Expect(uh).To(BeFalse())
				Expect(ph).To(BeFalse())
				Expect(w.Code).To(Equal(http.StatusInternalServerError))
				Expect(w.Body.String()).To(Equal("Internal Server Error"))
			})
		})

		When("route is not found", func() {
			It("routes to the probe handler", func() {
				const (
					uaKey = "User-Agent"
					uaVal = "kube-probe/0"
				)

				var (
					sc = http.StatusTeapot
					st = http.StatusText(sc)
				)

				var uh bool
				upstreamHandler.Handle(path, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					uh = true
				}))

				var ph bool
				probeHandler.Handle(path, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusTeapot)

					_, err := w.Write([]byte(st))
					Expect(err).NotTo(HaveOccurred())

					ph = true
				}))

				r.Header.Set(uaKey, uaVal)

				routingMiddleware.ServeHTTP(w, r)

				Expect(uh).To(BeFalse())
				Expect(ph).To(BeTrue())
				Expect(w.Code).To(Equal(sc))
				Expect(w.Body.String()).To(Equal(st))
			})

			It("serves 404", func() {
				var (
					sc = http.StatusNotFound
					st = http.StatusText(sc)
				)

				var uh bool
				upstreamHandler.Handle(path, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					uh = true
				}))

				var ph bool
				probeHandler.Handle(path, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					ph = true
				}))

				routingMiddleware.ServeHTTP(w, r)

				Expect(uh).To(BeFalse())
				Expect(ph).To(BeFalse())
				Expect(w.Code).To(Equal(sc))
				Expect(w.Body.String()).To(Equal(st))
			})
		})
	})

	Context("isKubeProbe", func() {
		const (
			uaKey = "User-Agent"
		)

		var (
			r *http.Request
		)

		BeforeEach(func() {
			r = httptest.NewRequest(http.MethodGet, "/", nil)
		})

		It("returns true if the request is from kube-probe", func() {
			const (
				uaVal = "Go-http-client/1.1 kube-probe/1.27.1 (linux/amd64) kubernetes/4c94112"
			)

			r.Header.Set(uaKey, uaVal)

			var rm Routing
			b := rm.isProbe(r)
			Expect(b).To(BeTrue())
		})

		It("returns true if the request is from GoogleHC", func() {
			const (
				uaVal = "Go-http-client/1.1 GoogleHC/1.0 (linux/amd64) kubernetes/4c94112"
			)

			r.Header.Set(uaKey, uaVal)

			var rm Routing
			b := rm.isProbe(r)
			Expect(b).To(BeTrue())
		})

		It("returns false if the request is not from kube-probe or GoogleHC", func() {
			const (
				uaVal = "Go-http-client/1.1 kubectl/v1.27.1 (linux/amd64) kubernetes/4c94112"
			)

			r.Header.Set(uaKey, uaVal)

			var rm Routing
			b := rm.isProbe(r)
			Expect(b).To(BeFalse())
		})
	})
})
