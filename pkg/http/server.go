package http

import (
	"context"
	"crypto/tls"
	"net/http"

	"github.com/kedacore/http-add-on/interceptor/config"
	"github.com/kedacore/http-add-on/pkg/util"
)

func ServeContext(
	ctx context.Context,
	addr string,
	hdl http.Handler,
	tlsConfig *tls.Config,
	timeouts *config.Timeouts,
) error {
	srv := &http.Server{
		Handler:      hdl,
		Addr:         addr,
		TLSConfig:    tlsConfig,
		ReadTimeout:  timeouts.ServerReadTimeout,
		WriteTimeout: timeouts.ServerWriteTimeout,
		IdleTimeout:  timeouts.ServerIdleTimeout,
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
