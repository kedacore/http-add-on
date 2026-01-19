package middleware

import (
	"net/http"
)

// HijackerResponseWriter combines http.ResponseWriter, http.Hijacker and EnableFullDuplex interfaces
// This is used for generating mocks that implement all interfaces
type HijackerResponseWriter interface {
	http.ResponseWriter
	http.Hijacker
	EnableFullDuplex() error
}
