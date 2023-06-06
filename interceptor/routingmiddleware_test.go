package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"

	"github.com/go-logr/logr/funcr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	httpv1alpha1 "github.com/kedacore/http-add-on/operator/apis/http/v1alpha1"
	"github.com/kedacore/http-add-on/pkg/routing"
)

var _ = Describe("RoutingMiddleware", func() {
	Context("New", func() {
		It("returns new object with expected fields", func() {
			var (
				routingTable    = newTestRoutingTable()
				probeHandler    = http.NewServeMux()
				upstreamHandler = http.NewServeMux()
			)
			probeHandler.Handle("/probe", http.HandlerFunc(nil))
			upstreamHandler.Handle("/upstream", http.HandlerFunc(nil))

			rm := NewRoutingMiddleware(routingTable, probeHandler, upstreamHandler)
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
			routingTable      *testRoutingTable
			routingMiddleware *RoutingMiddleware
			w                 *httptest.ResponseRecorder
			r                 *http.Request

			httpso = httpv1alpha1.HTTPScaledObject{
				Spec: httpv1alpha1.HTTPScaledObjectSpec{
					Hosts: []string{
						host,
					},
				},
			}
		)

		BeforeEach(func() {
			upstreamHandler = http.NewServeMux()
			probeHandler = http.NewServeMux()
			routingTable = newTestRoutingTable()
			routingMiddleware = NewRoutingMiddleware(routingTable, probeHandler, upstreamHandler)

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

				routingTable.memory[host] = &httpso

				routingMiddleware.ServeHTTP(w, r)

				Expect(uh).To(BeTrue())
				Expect(ph).To(BeFalse())
				Expect(w.Code).To(Equal(sc))
				Expect(w.Body.String()).To(Equal(st))
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

			var rm RoutingMiddleware
			b := rm.isKubeProbe(r)
			Expect(b).To(BeTrue())
		})

		It("returns false if the request is not from kube-probe", func() {
			const (
				uaVal = "Go-http-client/1.1 kubectl/v1.27.1 (linux/amd64) kubernetes/4c94112"
			)

			r.Header.Set(uaKey, uaVal)

			var rm RoutingMiddleware
			b := rm.isKubeProbe(r)
			Expect(b).To(BeFalse())
		})
	})

	Context("serveNotFound", func() {
		var (
			w *httptest.ResponseRecorder
			r *http.Request

			sc = http.StatusNotFound
			st = http.StatusText(sc)
		)

		BeforeEach(func() {
			w = httptest.NewRecorder()
			r = httptest.NewRequest(http.MethodGet, "/", nil)
		})

		It("serves a 404 response", func() {
			var rm RoutingMiddleware
			rm.serveNotFound(w, r)

			Expect(w.Code).To(Equal(sc))
			Expect(w.Body.String()).To(Equal(st))
		})

		It("logs the failed request", func() {
			var b bool
			r = r.WithContext(context.WithValue(r.Context(), ContextKeyLogger, funcr.NewJSON(
				func(obj string) {
					var m map[string]interface{}

					err := json.Unmarshal([]byte(obj), &m)
					Expect(err).NotTo(HaveOccurred())

					rk := routing.NewKeyFromRequest(r)
					Expect(m).To(HaveKeyWithValue("msg", http.StatusText(http.StatusNotFound)))
					Expect(m).To(HaveKeyWithValue("routingKey", rk.String()))

					b = true
				},
				funcr.Options{},
			)))

			var rm RoutingMiddleware
			rm.serveNotFound(w, r)

			Expect(b).To(BeTrue())
		})
	})
})
