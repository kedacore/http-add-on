package http

import (
	"context"
	"net/http"
)

func ServeContext(ctx context.Context, addr string, hdl http.Handler) error {
	srv := &http.Server{
		Handler: hdl,
		Addr:    addr,
	}

	go func() {
		<-ctx.Done()
		srv.Shutdown(ctx)
	}()
	return srv.ListenAndServe()
}
