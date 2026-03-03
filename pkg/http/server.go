package http

import (
	"context"
	"crypto/tls"
	"net/http"
	"time"

	"github.com/kedacore/http-add-on/pkg/util"
)

func ServeContext(ctx context.Context, addr string, hdl http.Handler, tlsConfig *tls.Config) error {
	srv := &http.Server{
		Handler:           hdl,
		Addr:              addr,
		TLSConfig:         tlsConfig,
		ReadHeaderTimeout: time.Minute, // mitigate Slowloris attacks
	}

	go func() {
		<-ctx.Done()

		if err := srv.Shutdown(context.Background()); err != nil {
			logger := util.LoggerFromContext(ctx)
			logger.Error(err, "failed shutting down server")
		}
	}()

	if tlsConfig != nil {
		return srv.ListenAndServeTLS("", "")
	}

	return srv.ListenAndServe()
}
