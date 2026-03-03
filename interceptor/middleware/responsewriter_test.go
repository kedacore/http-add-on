package middleware

import (
	"bufio"
	"fmt"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
)

var _ = Describe("responseWriter", func() {
	Context("Interface compliance", func() {
		It("implements http.ResponseWriter interface", func() {
			var rw *responseWriter
			var _ http.ResponseWriter = rw
		})

		It("implements http.Hijacker interface", func() {
			var rw *responseWriter
			var _ http.Hijacker = rw
		})
	})

	Context("New", func() {
		It("returns new object with expected field values set", func() {
			w := httptest.NewRecorder()

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
			w := httptest.NewRecorder()

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

			w := httptest.NewRecorder()

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

			w := httptest.NewRecorder()

			rw := &responseWriter{
				statusCode:               http.StatusOK,
				downstreamResponseWriter: w,
			}
			rw.WriteHeader(sc)

			Expect(rw.statusCode).To(Equal(sc))

			Expect(w.Code).To(Equal(sc))
		})
	})

	Context("Hijack", func() {
		var ctrl *gomock.Controller

		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())
		})

		AfterEach(func() {
			ctrl.Finish()
		})

		It("successfully hijacks when downstream ResponseWriter implements http.Hijacker", func() {
			// Create mocks using the generated mocks
			mockConn := NewMockConn(ctrl)
			mockReadWriter := &bufio.ReadWriter{}
			mockHijackerWriter := NewMockHijackerResponseWriter(ctrl)

			// Set up expectations
			mockHijackerWriter.EXPECT().Hijack().Return(mockConn, mockReadWriter, nil)

			rw := &responseWriter{
				downstreamResponseWriter: mockHijackerWriter,
			}

			conn, readWriter, err := rw.Hijack()

			Expect(err).To(BeNil())
			Expect(conn).To(Equal(mockConn))
			Expect(readWriter).To(Equal(mockReadWriter))
		})

		It("returns error when downstream ResponseWriter does not implement http.Hijacker", func() {
			w := httptest.NewRecorder()

			rw := &responseWriter{
				downstreamResponseWriter: w,
			}

			conn, readWriter, err := rw.Hijack()

			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(Equal("http.Hijacker not implemented"))
			Expect(conn).To(BeNil())
			Expect(readWriter).To(BeNil())
		})

		It("forwards error when downstream hijacker returns error", func() {
			expectedError := fmt.Errorf("hijack failed")
			mockHijackerWriter := NewMockHijackerResponseWriter(ctrl)

			// Set up expectations
			mockHijackerWriter.EXPECT().Hijack().Return(nil, nil, expectedError)

			rw := &responseWriter{
				downstreamResponseWriter: mockHijackerWriter,
			}

			conn, readWriter, err := rw.Hijack()

			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(Equal("hijack failed"))
			Expect(conn).To(BeNil())
			Expect(readWriter).To(BeNil())
		})
	})

	Context("Full Duplex", func() {
		var ctrl *gomock.Controller

		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())
		})

		AfterEach(func() {
			ctrl.Finish()
		})

		It("successfully enables full duplex when downstream ResponseWriter supports it", func() {
			// Create mocks using the generated mocks
			mockWriter := NewMockHijackerResponseWriter(ctrl)

			// Set up expectations
			mockWriter.EXPECT().EnableFullDuplex().Return(nil)

			rw := &responseWriter{
				downstreamResponseWriter: mockWriter,
			}

			err := rw.EnableFullDuplex()

			Expect(err).To(BeNil())
		})

		It("returns error when downstream ResponseWriter does not support full duplex", func() {
			w := httptest.NewRecorder()

			rw := &responseWriter{
				downstreamResponseWriter: w,
			}

			err := rw.EnableFullDuplex()

			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(Equal("EnableFullDuplex() not implemented"))
		})

		It("forwards error when downstream hijacker returns error", func() {
			expectedError := fmt.Errorf("enable full duplex failed")
			mockWriter := NewMockHijackerResponseWriter(ctrl)

			// Set up expectations
			mockWriter.EXPECT().EnableFullDuplex().Return(expectedError)

			rw := &responseWriter{
				downstreamResponseWriter: mockWriter,
			}

			err := rw.EnableFullDuplex()

			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(Equal("enable full duplex failed"))
		})
	})
})
