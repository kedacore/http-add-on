package http

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// ServerConfig holds the parameters for ListenAndServe.
type ServerConfig struct {
	Addr         string
	Handler      http.Handler
	TLSConfig    *tls.Config
	DrainTimeout time.Duration
	Draining     *atomic.Bool
}

// ServeContext creates a TCP listener on Addr and serves HTTP(S) requests
// until ctx is cancelled. When ctx is cancelled, it initiates a graceful
// shutdown, waiting up to DrainTimeout for in-flight requests to complete.
// If DrainTimeout is 0, it waits indefinitely.
//
// While Draining is true, HTTP/1.x responses include a "Connection: close"
// header, signalling clients to stop reusing the connection.
func ServeContext(ctx context.Context, cfg ServerConfig) error {
	ln, err := net.Listen("tcp", cfg.Addr)
	if err != nil {
		return fmt.Errorf("listening on %s: %w", cfg.Addr, err)
	}
	return serve(ctx, ln, cfg)
}

// serve creates an HTTP server with handlers and mechanisms to drain open connections on shutdown.
// The whole draining logic is quite complex e.g. as WebSockets can't be drained like normal HTTP connections.
// HTTP/1.x clients are signalled via "Connection: close" once Draining is set.
func serve(ctx context.Context, ln net.Listener, cfg ServerConfig) error {
	// Provide a local cancel for the context to trigger a shutdown from inside if the server stops e.g. due to errors.
	ctx, cancel := context.WithCancel(ctx)

	// WebSocket connections are so-called hijacked HTTP connections that we need to 1. track and 2. cancel separately.
	// 1. http.Server.Shutdown() does not wait for hijacked connections -> track them with a WaitGroup to allow
	// waiting for all open connections to complete.
	var connectionTracker sync.WaitGroup
	trackingHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		connectionTracker.Add(1)
		defer connectionTracker.Done()
		cfg.Handler.ServeHTTP(w, r)
	})

	var handler http.Handler = trackingHandler
	if cfg.Draining != nil {
		handler = newDrainHandler(trackingHandler, cfg.Draining)
	}

	// 2. Create a BaseContext that can be used to cancel hijacked connections (e.g. WebSockets) after the
	// http.Server.Shutdown() was triggered.
	baseCtx, cancelBase := context.WithCancel(context.Background())
	defer cancelBase()

	var protocols http.Protocols
	protocols.SetHTTP1(true)
	protocols.SetHTTP2(true)
	protocols.SetUnencryptedHTTP2(true)

	// Create the actual server
	srv := &http.Server{
		Handler:           handler,
		TLSConfig:         cfg.TLSConfig,
		Protocols:         &protocols,
		ReadHeaderTimeout: time.Minute, // mitigate Slowloris attacks
		BaseContext: func(_ net.Listener) context.Context {
			return baseCtx
		},
	}

	// Define the graceful shutdown logic that triggers and waits for the draining.
	shutdownDone := make(chan struct{})
	go func() { //nolint:gosec // G118: ctx is already cancelled here; shutdown needs a fresh context
		defer close(shutdownDone)

		// Wait for the cancellation of the parent context or from this func in case of server errors.
		<-ctx.Done()

		drainCtx := context.Background()
		if cfg.DrainTimeout > 0 {
			var drainCancel context.CancelFunc
			drainCtx, drainCancel = context.WithTimeout(drainCtx, cfg.DrainTimeout)
			defer drainCancel()
		}

		// Drain regular connections first gracefully, signal hijacked handlers only afterwards.
		_ = srv.Shutdown(drainCtx)
		cancelBase()

		// Wait for remaining (hijacked) handlers to finish, bounded by the drain timeout.
		ch := make(chan struct{})
		go func() { connectionTracker.Wait(); close(ch) }()
		select {
		case <-ch:
		case <-drainCtx.Done():
		}
	}()

	// Serve blocks until Shutdown closes the listener or a listener error occurs.
	var serveErr error
	if cfg.TLSConfig != nil {
		serveErr = srv.ServeTLS(ln, "", "")
	} else {
		serveErr = srv.Serve(ln)
	}

	// Unblock the shutdown goroutine if Serve returned without ctx being cancelled (e.g. listener error).
	cancel()
	<-shutdownDone

	if errors.Is(serveErr, http.ErrServerClosed) {
		return nil
	}
	return serveErr
}
