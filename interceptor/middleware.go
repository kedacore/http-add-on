package main

import (
	"log"

	"github.com/kedacore/http-add-on/pkg/http"
	"github.com/labstack/echo/v4"
)

// countMiddleware takes que MemoryQueue previously initiated and increments the
// size of it before sending the request to the original app, after the request
// is finished, it decrements the queue size
func countMiddleware(q http.QueueCounter) echo.MiddlewareFunc {
	return func(fn echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// TODO: need to figure out a way to get the increment
			// to happen before fn(w, r) happens below. otherwise,
			// the counter won't get incremented right away and the actual
			// handler will hang longer than it needs to
			go func() {
				if err := q.Resize(+1); err != nil {
					log.Printf("Error incrementing queue for %q (%s)", c.Request().RequestURI, err)
				}
			}()
			defer func() {
				if err := q.Resize(-1); err != nil {
					log.Printf("Error decrementing queue for %q (%s)", c.Request().RequestURI, err)
				}
			}()
			fn(c)
			return nil
		}
	}
}
