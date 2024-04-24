package middleware

import (
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("responseWriter", func() {
	Context("New", func() {
		It("returns new object with expected field values set", func() {
			var (
				w = httptest.NewRecorder()
			)

			rw := newResponseWriter(w)
			Expect(rw).NotTo(BeNil())
			Expect(rw.downstreamResponseWriter).To(Equal(w))
			Expect(rw.bytesWritten).To(Equal(0))
			Expect(rw.statusCode).To(Equal(0))
		})
	})

	Context("BytesWritten", func() {
		It("returns the expected value", func() {
			const (
				bw = 128
			)

			rw := &responseWriter{
				bytesWritten: bw,
			}

			ret := rw.BytesWritten()
			Expect(ret).To(Equal(bw))
		})
	})

	Context("StatusCode", func() {
		It("returns the expected value", func() {
			const (
				sc = http.StatusTeapot
			)

			rw := &responseWriter{
				statusCode: sc,
			}

			ret := rw.StatusCode()
			Expect(ret).To(Equal(sc))
		})
	})

	Context("Header", func() {
		It("returns downstream method call", func() {
			var (
				w = httptest.NewRecorder()
			)

			rw := &responseWriter{
				downstreamResponseWriter: w,
			}

			h := w.Header()
			h.Set("Content-Type", "application/json")

			ret := rw.Header()
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

			rw := &responseWriter{
				bytesWritten:             initialBW,
				downstreamResponseWriter: w,
			}

			n, err := rw.Write([]byte(body))
			Expect(err).To(BeNil())
			Expect(n).To(Equal(bodyLen))

			Expect(rw.bytesWritten).To(Equal(initialBW + bodyLen))

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

			rw := &responseWriter{
				statusCode:               http.StatusOK,
				downstreamResponseWriter: w,
			}
			rw.WriteHeader(sc)

			Expect(rw.statusCode).To(Equal(sc))

			Expect(w.Code).To(Equal(sc))
		})
	})
})
