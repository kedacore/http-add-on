package util

import (
	"context"
	"net/url"

	"github.com/go-logr/logr"

	httpv1alpha1 "github.com/kedacore/http-add-on/operator/apis/http/v1alpha1"
)

type contextKey int

const (
	ckLogger contextKey = iota
	ckHTTPSO
	ckStream
)

func ContextWithLogger(ctx context.Context, logger logr.Logger) context.Context {
	return context.WithValue(ctx, ckLogger, logger)
}

func LoggerFromContext(ctx context.Context) logr.Logger {
	cv, _ := ctx.Value(ckLogger).(logr.Logger)
	return cv
}

func ContextWithHTTPSO(ctx context.Context, httpso *httpv1alpha1.HTTPScaledObject) context.Context {
	return context.WithValue(ctx, ckHTTPSO, httpso)
}

func HTTPSOFromContext(ctx context.Context) *httpv1alpha1.HTTPScaledObject {
	cv, _ := ctx.Value(ckHTTPSO).(*httpv1alpha1.HTTPScaledObject)
	return cv
}

func ContextWithStream(ctx context.Context, url *url.URL) context.Context {
	return context.WithValue(ctx, ckStream, url)
}

func StreamFromContext(ctx context.Context) *url.URL {
	cv, _ := ctx.Value(ckStream).(*url.URL)
	return cv
}
