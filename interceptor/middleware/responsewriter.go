package middleware

import (
	"net/http"
)

// instrumentedResponseWriter wraps http.ResponseWriter to capture the status code and bytes written
// for logging and metrics.
// It implements Unwrap() so that we don't have to reimplement optional interfaces like Flusher, Hijacker, ...
type instrumentedResponseWriter struct {
	http.ResponseWriter
	bytesWritten int
	statusCode   int
}

func newInstrumentedResponseWriter(w http.ResponseWriter) *instrumentedResponseWriter {
	return &instrumentedResponseWriter{
		ResponseWriter: w,
		statusCode:     http.StatusOK, // default value if WriteHeader is not called
	}
}

var _ interface{ Unwrap() http.ResponseWriter } = (*instrumentedResponseWriter)(nil)

func (rw *instrumentedResponseWriter) Unwrap() http.ResponseWriter {
	return rw.ResponseWriter
}

func (rw *instrumentedResponseWriter) Write(bytes []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(bytes)
	rw.bytesWritten += n

	return n, err
}

func (rw *instrumentedResponseWriter) WriteHeader(statusCode int) {
	rw.ResponseWriter.WriteHeader(statusCode)
	rw.statusCode = statusCode
}
