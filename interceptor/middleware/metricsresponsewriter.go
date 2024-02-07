package middleware

import (
	"net/http"
)

type metricsResponseWriter struct {
	downstreamResponseWriter http.ResponseWriter
	bytesWritten             int
	statusCode               int
}

func newMetricsResponseWriter(downstreamResponseWriter http.ResponseWriter) *metricsResponseWriter {
	return &metricsResponseWriter{
		downstreamResponseWriter: downstreamResponseWriter,
	}
}

func (mrw *metricsResponseWriter) BytesWritten() int {
	return mrw.bytesWritten
}

func (mrw *metricsResponseWriter) StatusCode() int {
	return mrw.statusCode
}

var _ http.ResponseWriter = (*metricsResponseWriter)(nil)

func (mrw *metricsResponseWriter) Header() http.Header {
	return mrw.downstreamResponseWriter.Header()
}

func (mrw *metricsResponseWriter) Write(bytes []byte) (int, error) {
	n, err := mrw.downstreamResponseWriter.Write(bytes)
	if f, ok := mrw.downstreamResponseWriter.(http.Flusher); ok {
		f.Flush()
	}

	mrw.bytesWritten += n

	return n, err
}

func (mrw *metricsResponseWriter) WriteHeader(statusCode int) {
	mrw.downstreamResponseWriter.WriteHeader(statusCode)

	mrw.statusCode = statusCode
}
