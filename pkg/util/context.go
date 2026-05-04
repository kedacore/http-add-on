package util

import (
	"context"
	"net/url"

	"github.com/go-logr/logr"

	httpv1beta1 "github.com/kedacore/http-add-on/operator/apis/http/v1beta1"
)

type contextKey int

const (
	ckLogger contextKey = iota
	ckUpstreamURL
	ckFallbackURL
	ckIR
	ckUpstreamServerName
)

func ContextWithLogger(ctx context.Context, logger logr.Logger) context.Context {
	return context.WithValue(ctx, ckLogger, logger)
}

func LoggerFromContext(ctx context.Context) logr.Logger {
	cv, _ := ctx.Value(ckLogger).(logr.Logger)
	return cv
}

func ContextWithInterceptorRoute(ctx context.Context, ir *httpv1beta1.InterceptorRoute) context.Context {
	return context.WithValue(ctx, ckIR, ir)
}

func InterceptorRouteFromContext(ctx context.Context) *httpv1beta1.InterceptorRoute {
	cv, _ := ctx.Value(ckIR).(*httpv1beta1.InterceptorRoute)
	return cv
}

func ContextWithUpstreamURL(ctx context.Context, url *url.URL) context.Context {
	return context.WithValue(ctx, ckUpstreamURL, url)
}

func UpstreamURLFromContext(ctx context.Context) *url.URL {
	cv, _ := ctx.Value(ckUpstreamURL).(*url.URL)
	return cv
}

func ContextWithFallbackURL(ctx context.Context, url *url.URL) context.Context {
	return context.WithValue(ctx, ckFallbackURL, url)
}

func FallbackURLFromContext(ctx context.Context) *url.URL {
	cv, _ := ctx.Value(ckFallbackURL).(*url.URL)
	return cv
}

// ContextWithUpstreamServerName stores the intended TLS server name (SNI) for the upstream.
// This must be set before any middleware rewrites the upstream URL (e.g. direct-to-pod),
func ContextWithUpstreamServerName(ctx context.Context, serverName string) context.Context {
	return context.WithValue(ctx, ckUpstreamServerName, serverName)
}

func UpstreamServerNameFromContext(ctx context.Context) string {
	cv, _ := ctx.Value(ckUpstreamServerName).(string)
	return cv
}
