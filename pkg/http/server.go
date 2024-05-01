package http

import (
	"context"
	"net/http"

	"github.com/kedacore/http-add-on/pkg/util"
)

func ServeContext(ctx context.Context, addr string, hdl http.Handler, tlsEnabled bool, tlsConfig map[string]string) error {
	srv := &http.Server{
		Handler: hdl,
		Addr:    addr,
	}

	go func() {
		<-ctx.Done()

		if err := srv.Shutdown(context.Background()); err != nil {
			logger := util.LoggerFromContext(ctx)
			logger.Error(err, "failed shutting down server")
		}
	}()

	if tlsEnabled {
		return srv.ListenAndServeTLS(tlsConfig["certificatePath"], tlsConfig["keyPath"])
	}

	return srv.ListenAndServe()
}
