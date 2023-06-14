package util

import (
	"net/http"
	"net/url"

	"github.com/go-logr/logr"

	httpv1alpha1 "github.com/kedacore/http-add-on/operator/apis/http/v1alpha1"
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

func RequestWithHTTPSO(r *http.Request, httpso *httpv1alpha1.HTTPScaledObject) *http.Request {
	ctx := r.Context()
	ctx = ContextWithHTTPSO(ctx, httpso)

	return r.WithContext(ctx)
}

func RequestWithStream(r *http.Request, stream *url.URL) *http.Request {
	ctx := r.Context()
	ctx = ContextWithStream(ctx, stream)

	return r.WithContext(ctx)
}
