package main

import (
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("LoggingResponseWriter", func() {
	Context("New", func() {
		It("returns new object with expected field values set", func() {
			var (
				w = httptest.NewRecorder()
			)

			lrw := NewLoggingResponseWriter(w)
			Expect(lrw).NotTo(BeNil())
			Expect(lrw.downstreamResponseWriter).To(Equal(w))
			Expect(lrw.bytesWritten).To(Equal(0))
			Expect(lrw.statusCode).To(Equal(0))
		})
	})

	Context("BytesWritten", func() {
		It("returns the expected value", func() {
			const (
				bw = 128
			)

			lrw := &LoggingResponseWriter{
				bytesWritten: bw,
			}

			ret := lrw.BytesWritten()
			Expect(ret).To(Equal(bw))
		})
	})

	Context("StatusCode", func() {
		It("returns the expected value", func() {
			const (
				sc = http.StatusTeapot
			)

			lrw := &LoggingResponseWriter{
				statusCode: sc,
			}

			ret := lrw.StatusCode()
			Expect(ret).To(Equal(sc))
		})
	})

	Context("Header", func() {
		It("returns downstream method call", func() {
			var (
				w = httptest.NewRecorder()
			)

			lrw := &LoggingResponseWriter{
				downstreamResponseWriter: w,
			}

			h := w.Header()
			h.Set("Content-Type", "application/json")

			ret := lrw.Header()
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

			lrw := &LoggingResponseWriter{
				bytesWritten:             initialBW,
				downstreamResponseWriter: w,
			}

			n, err := lrw.Write([]byte(body))
			Expect(err).To(BeNil())
			Expect(n).To(Equal(bodyLen))

			Expect(lrw.bytesWritten).To(Equal(initialBW + bodyLen))

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

			lrw := &LoggingResponseWriter{
				statusCode:               http.StatusOK,
				downstreamResponseWriter: w,
			}
			lrw.WriteHeader(sc)

			Expect(lrw.statusCode).To(Equal(sc))

			Expect(w.Code).To(Equal(sc))
		})
	})
})
