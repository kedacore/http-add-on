package middleware

import (
	"net/http"
)

type loggingResponseWriter struct {
	downstreamResponseWriter http.ResponseWriter
	bytesWritten             int
	statusCode               int
}

func newLoggingResponseWriter(downstreamResponseWriter http.ResponseWriter) *loggingResponseWriter {
	return &loggingResponseWriter{
		downstreamResponseWriter: downstreamResponseWriter,
	}
}

func (lrw *loggingResponseWriter) BytesWritten() int {
	return lrw.bytesWritten
}

func (lrw *loggingResponseWriter) StatusCode() int {
	return lrw.statusCode
}

var _ http.ResponseWriter = (*loggingResponseWriter)(nil)

func (lrw *loggingResponseWriter) Header() http.Header {
	return lrw.downstreamResponseWriter.Header()
}

func (lrw *loggingResponseWriter) Write(bytes []byte) (int, error) {
	n, err := lrw.downstreamResponseWriter.Write(bytes)
	if f, ok := lrw.downstreamResponseWriter.(http.Flusher); ok {
		f.Flush()
	}

	lrw.bytesWritten += n

	return n, err
}

func (lrw *loggingResponseWriter) WriteHeader(statusCode int) {
	lrw.downstreamResponseWriter.WriteHeader(statusCode)

	lrw.statusCode = statusCode
}
