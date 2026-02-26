package middleware

import (
	"bufio"
	"errors"
	"net"
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

var (
	_ http.ResponseWriter = (*responseWriter)(nil)
	_ http.Hijacker       = (*responseWriter)(nil)
)

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

func (rw *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hj, ok := rw.downstreamResponseWriter.(http.Hijacker); ok {
		return hj.Hijack()
	}

	return nil, nil, errors.New("http.Hijacker not implemented")
}

// https://pkg.go.dev/net/http#ResponseController.EnableFullDuplex
func (rw *responseWriter) EnableFullDuplex() error {
	if hj, ok := rw.downstreamResponseWriter.(interface{ EnableFullDuplex() error }); ok {
		return hj.EnableFullDuplex()
	}

	return errors.New("EnableFullDuplex() not implemented")
}
