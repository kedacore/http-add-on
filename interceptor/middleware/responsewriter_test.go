package middleware

import (
	"net/http"
	"net/http/httptest"

	"github.com/gorilla/websocket"
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

	Context("Websocket", func() {
		It("returns the expected values when http.Hijacker is implemented", func() {

			// Create a server that will accept websocket connections
			w := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Upgrade the connection to a websocket connection
				upgrader := websocket.Upgrader{}
				conn, err := upgrader.Upgrade(w, r, nil)
				Expect(err).To(BeNil())
				Expect(conn).NotTo(BeNil())

				// Write a message to the client
				err = conn.WriteMessage(websocket.TextMessage, []byte("hello"))
				Expect(err).To(BeNil())

				// Close the connection
				err = conn.Close()
				Expect(err).To(BeNil())
			}))
			defer w.Close()

			w.Client()

			lrw := &loggingResponseWriter{
				downstreamResponseWriter: w.Client(),
			}

			// Create a client that will connect to the server
			dialer := websocket.Dialer{}
			conn, _, err := dialer.Dial(w.URL, nil)
		})

		It("returns an error when http.Hijacker is not implemented", func() {
			w := httptest.NewRecorder()
			lrw := &loggingResponseWriter{
				downstreamResponseWriter: w,
			}

			c, r, err := lrw.Hijack()
			Expect(err).To(MatchError("http.Hijacker not implemented"))
			Expect(c).To(BeNil())
			Expect(r).To(BeNil())
		})
	})
})
