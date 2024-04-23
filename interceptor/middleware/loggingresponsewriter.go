package middleware

import (
	"bufio"
	"errors"
	"net"
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

// implements http.hijacker
func (lrw *loggingResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hj, ok := lrw.downstreamResponseWriter.(http.Hijacker); ok {
		return hj.Hijack()
	}

	return nil, nil, errors.New("http.Hijacker not implemented")
}
