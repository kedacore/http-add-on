package middleware

import "context"

type contextKeyType int

const routeInfoKey contextKeyType = iota

// routeInfo carries route identity through the middleware chain via a shared
// pointer in the request context.
// The zero value represents an unmatched request.
type routeInfo struct {
	Name      string
	Namespace string
	// IsColdStart is true when the request waited for backend readiness,
	// including waits that ended before the backend became ready.
	IsColdStart bool
}

func contextWithRouteInfo(ctx context.Context, info *routeInfo) context.Context {
	return context.WithValue(ctx, routeInfoKey, info)
}

func routeInfoFromContext(ctx context.Context) *routeInfo {
	ri, _ := ctx.Value(routeInfoKey).(*routeInfo)
	return ri
}
