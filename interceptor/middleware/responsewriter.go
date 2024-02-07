package middleware

import (
	"net/http"
)

type responseWriter struct {
	downstreamResponseWriter http.ResponseWriter
	bytesWritten             int
	statusCode               int
}

func newResponseWriter(downstreamResponseWriter http.ResponseWriter) *responseWriter {
	return &responseWriter{
		downstreamResponseWriter: downstreamResponseWriter,
	}
}

func (rw *responseWriter) BytesWritten() int {
	return rw.bytesWritten
}

func (rw *responseWriter) StatusCode() int {
	return rw.statusCode
}

var _ http.ResponseWriter = (*responseWriter)(nil)

func (rw *responseWriter) Header() http.Header {
	return rw.downstreamResponseWriter.Header()
}

func (rw *responseWriter) Write(bytes []byte) (int, error) {
	n, err := rw.downstreamResponseWriter.Write(bytes)
	if f, ok := rw.downstreamResponseWriter.(http.Flusher); ok {
		f.Flush()
	}

	rw.bytesWritten += n

	return n, err
}

func (rw *responseWriter) WriteHeader(statusCode int) {
	rw.downstreamResponseWriter.WriteHeader(statusCode)

	rw.statusCode = statusCode
}
