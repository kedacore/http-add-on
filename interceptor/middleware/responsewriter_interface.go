package middleware

import (
	"net/http"
)

// HijackerResponseWriter combines http.ResponseWriter and http.Hijacker interfaces
// This is used for generating mocks that implement both interfaces
type HijackerResponseWriter interface {
	http.ResponseWriter
	http.Hijacker
}
