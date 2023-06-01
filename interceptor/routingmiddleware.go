package main

import (
	"context"
	"net/http"
	"regexp"

	"github.com/go-logr/logr"

	"github.com/kedacore/http-add-on/pkg/routing"
)

var (
	kpUserAgent = regexp.MustCompile(`(^|\s)kube-probe/`)
)

type RoutingMiddleware struct {
	routingTable    routing.Table
	probeHandler    http.Handler
	upstreamHandler http.Handler
}

func NewRoutingMiddleware(routingTable routing.Table, probeHandler http.Handler, upstreamHandler http.Handler) *RoutingMiddleware {
	return &RoutingMiddleware{
		routingTable:    routingTable,
		probeHandler:    probeHandler,
		upstreamHandler: upstreamHandler,
	}
}

var _ http.Handler = (*RoutingMiddleware)(nil)

func (rm *RoutingMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	httpso := rm.routingTable.Route(r)
	if httpso == nil {
		if rm.isKubeProbe(r) {
			rm.probeHandler.ServeHTTP(w, r)
			return
		}

		rm.serveNotFound(w, r)
		return
	}

	ctx = context.WithValue(ctx, HTTPSOContextKey, httpso)
	r = r.WithContext(ctx)

	rm.upstreamHandler.ServeHTTP(w, r)
}

func (rm *RoutingMiddleware) isKubeProbe(r *http.Request) bool {
	ua := r.UserAgent()
	return kpUserAgent.Match([]byte(ua))
}

func (rm *RoutingMiddleware) serveNotFound(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := ctx.Value(LoggerContextKey).(logr.Logger)

	sc := http.StatusNotFound
	st := http.StatusText(sc)

	routingKey := routing.NewKeyFromRequest(r)
	logger.Error(nil, st, "routingKey", routingKey)

	w.WriteHeader(sc)
	if _, err := w.Write([]byte(st)); err != nil {
		logger.Error(err, "write failed")
	}
}
