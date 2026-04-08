package http

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/kedacore/http-add-on/pkg/util"
)

// ServeContext creates a TCP listener on addr and serves HTTP(S) requests until ctx is cancelled.
func ServeContext(ctx context.Context, addr string, hdl http.Handler, tlsConfig *tls.Config) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listening on %s: %w", addr, err)
	}
	return serve(ctx, ln, hdl, tlsConfig)
}

func serve(ctx context.Context, ln net.Listener, hdl http.Handler, tlsConfig *tls.Config) error {
	srv := &http.Server{
		Handler:           hdl,
		TLSConfig:         tlsConfig,
		ReadHeaderTimeout: time.Minute, // mitigate Slowloris attacks
	}

	go func() { //nolint:gosec // G118: ctx is already cancelled here; shutdown needs a fresh context
		<-ctx.Done()

		if err := srv.Shutdown(context.Background()); err != nil {
			logger := util.LoggerFromContext(ctx)
			logger.Error(err, "failed shutting down server")
		}
	}()

	if tlsConfig != nil {
		return srv.ServeTLS(ln, "", "")
	}
	return srv.Serve(ln)
}
