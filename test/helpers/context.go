//go:build e2e

package helpers

import (
	"context"
	"crypto/x509"

	"sigs.k8s.io/e2e-framework/klient"
)

type contextKey string

const (
	clientKey    contextKey = "client"
	namespaceKey contextKey = "namespace"
	proxyAddrKey contextKey = "proxyAddr"
	caIssuerKey  contextKey = "caIssuer"
	caPoolKey    contextKey = "caPool"
)

func contextWithClient(ctx context.Context, client klient.Client) context.Context {
	return context.WithValue(ctx, clientKey, client)
}

func clientFromContext(ctx context.Context) klient.Client {
	return ctx.Value(clientKey).(klient.Client)
}

func contextWithNamespace(ctx context.Context, ns string) context.Context {
	return context.WithValue(ctx, namespaceKey, ns)
}

func namespaceFromContext(ctx context.Context) string {
	return ctx.Value(namespaceKey).(string)
}

func contextWithProxyAddr(ctx context.Context, addr string) context.Context {
	return context.WithValue(ctx, proxyAddrKey, addr)
}

func proxyAddrFromContext(ctx context.Context) string {
	return ctx.Value(proxyAddrKey).(string)
}

func contextWithCAIssuer(ctx context.Context, issuerName string) context.Context {
	return context.WithValue(ctx, caIssuerKey, issuerName)
}

func caIssuerFromContext(ctx context.Context) string {
	v, _ := ctx.Value(caIssuerKey).(string)
	return v
}

func contextWithCAPool(ctx context.Context, pool *x509.CertPool) context.Context {
	return context.WithValue(ctx, caPoolKey, pool)
}

func caPoolFromContext(ctx context.Context) *x509.CertPool {
	v, _ := ctx.Value(caPoolKey).(*x509.CertPool)
	return v
}
