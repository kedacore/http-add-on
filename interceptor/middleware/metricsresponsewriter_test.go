package middleware

import (
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("metricsResponseWriter", func() {
	Context("New", func() {
		It("returns new object with expected field values set", func() {
			var (
				w = httptest.NewRecorder()
			)

			mrw := newMetricsResponseWriter(w)
			Expect(mrw).NotTo(BeNil())
			Expect(mrw.downstreamResponseWriter).To(Equal(w))
			Expect(mrw.bytesWritten).To(Equal(0))
			Expect(mrw.statusCode).To(Equal(0))
		})
	})

	Context("BytesWritten", func() {
		It("returns the expected value", func() {
			const (
				bw = 128
			)

			mrw := &metricsResponseWriter{
				bytesWritten: bw,
			}

			ret := mrw.BytesWritten()
			Expect(ret).To(Equal(bw))
		})
	})

	Context("StatusCode", func() {
		It("returns the expected value", func() {
			const (
				sc = http.StatusTeapot
			)

			mrw := &metricsResponseWriter{
				statusCode: sc,
			}

			ret := mrw.StatusCode()
			Expect(ret).To(Equal(sc))
		})
	})

	Context("Header", func() {
		It("returns downstream method call", func() {
			var (
				w = httptest.NewRecorder()
			)

			mrw := &metricsResponseWriter{
				downstreamResponseWriter: w,
			}

			h := w.Header()
			h.Set("Content-Type", "application/json")

			ret := mrw.Header()
			Expect(ret).To(Equal(h))
		})
	})

	Context("Write", func() {
		It("invokes downstream method, increases bytesWritten accordingly, and returns expected values", func() {
			const (
				body      = "KEDA"
				bodyLen   = len(body)
				initialBW = 60
			)

			var (
				w = httptest.NewRecorder()
			)

			mrw := &metricsResponseWriter{
				bytesWritten:             initialBW,
				downstreamResponseWriter: w,
			}

			n, err := mrw.Write([]byte(body))
			Expect(err).To(BeNil())
			Expect(n).To(Equal(bodyLen))

			Expect(mrw.bytesWritten).To(Equal(initialBW + bodyLen))

			Expect(w.Body.String()).To(Equal(body))
		})
	})

	Context("WriteHeader", func() {
		It("invokes downstream method and records the value", func() {
			const (
				sc = http.StatusTeapot
			)

			var (
				w = httptest.NewRecorder()
			)

			mrw := &metricsResponseWriter{
				statusCode:               http.StatusOK,
				downstreamResponseWriter: w,
			}
			mrw.WriteHeader(sc)

			Expect(mrw.statusCode).To(Equal(sc))

			Expect(w.Code).To(Equal(sc))
		})
	})
})
