package http

import (
	"context"
	"crypto/tls"
	"net/http"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"github.com/kedacore/http-add-on/pkg/util"
)

func ServeContext(ctx context.Context, addr string, hdl http.Handler, tlsConfig *tls.Config) error {
	// For non-TLS connections, wrap handler with h2c to support HTTP/2 cleartext
	if tlsConfig == nil {
		h2s := &http2.Server{}
		hdl = h2c.NewHandler(hdl, h2s)
	}

	srv := &http.Server{
		Handler:   hdl,
		Addr:      addr,
		TLSConfig: tlsConfig,
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
