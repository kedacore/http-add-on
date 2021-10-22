package http

import (
	"context"
	"fmt"
	"net/http"
)

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
