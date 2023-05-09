package http

import (
	"context"
	"fmt"
	"net/http"
)

// HostSwitch implements http.Handler. We need an object that implements the http.Handler interface.
// Therefore, we need a type for which we implement the ServeHTTP method.
// We just use a map here, in which we map host names (with port) to http.Handlers
type HostSwitch map[string]http.Handler

func ServeContext(ctx context.Context, addr string, hdl http.Handler) error {
	srv := &http.Server{
		Handler: hdl,
		Addr:    addr,
	}

	go func() {
		<-ctx.Done()
		if err := srv.Shutdown(ctx); err != nil {
			fmt.Println("failed shutting down server:", err)
		}
	}()
	return srv.ListenAndServe()
}

func (hs HostSwitch) ServeContext(ctx context.Context, addr string, hdl http.Handler) error {
	srv := &http.Server{
		Handler: hdl,
		Addr:    addr,
	}

	go func() {
		<-ctx.Done()
		if err := srv.Shutdown(ctx); err != nil {
			fmt.Println("failed shutting down server:", err)
		}
	}()
	return srv.ListenAndServe()
}
