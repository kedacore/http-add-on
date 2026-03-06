package util

import (
	"net/http"
	"net/url"

	"github.com/go-logr/logr"
)

func RequestWithLoggerWithName(r *http.Request, name string) *http.Request {
	logger := LoggerFromContext(r.Context())
	logger = logger.WithName(name)

	return RequestWithLogger(r, logger)
}

func RequestWithLogger(r *http.Request, logger logr.Logger) *http.Request {
	ctx := r.Context()
	ctx = ContextWithLogger(ctx, logger)

	return r.WithContext(ctx)
}

func RequestWithUpstreamURL(r *http.Request, url *url.URL) *http.Request {
	ctx := r.Context()
	ctx = ContextWithUpstreamURL(ctx, url)

	return r.WithContext(ctx)
}

func RequestWithFallbackURL(r *http.Request, url *url.URL) *http.Request {
	ctx := r.Context()
	ctx = ContextWithFallbackURL(ctx, url)

	return r.WithContext(ctx)
}
