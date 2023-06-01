package main

import (
	"net/http"
)

type LoggingResponseWriter struct {
	downstreamResponseWriter http.ResponseWriter
	bytesWritten             int
	statusCode               int
}

func NewLoggingResponseWriter(downstreamResponseWriter http.ResponseWriter) *LoggingResponseWriter {
	return &LoggingResponseWriter{
		downstreamResponseWriter: downstreamResponseWriter,
	}
}

func (lrw *LoggingResponseWriter) BytesWritten() int {
	return lrw.bytesWritten
}

func (lrw *LoggingResponseWriter) StatusCode() int {
	return lrw.statusCode
}

var _ http.ResponseWriter = (*LoggingResponseWriter)(nil)

func (lrw *LoggingResponseWriter) Header() http.Header {
	return lrw.downstreamResponseWriter.Header()
}

func (lrw *LoggingResponseWriter) Write(bytes []byte) (int, error) {
	n, err := lrw.downstreamResponseWriter.Write(bytes)

	lrw.bytesWritten += n

	return n, err
}

func (lrw *LoggingResponseWriter) WriteHeader(statusCode int) {
	lrw.downstreamResponseWriter.WriteHeader(statusCode)

	lrw.statusCode = statusCode
}
