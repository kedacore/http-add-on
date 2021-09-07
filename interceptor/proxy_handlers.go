package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/go-logr/logr"
	"github.com/kedacore/http-add-on/interceptor/config"
	kedanet "github.com/kedacore/http-add-on/pkg/net"
	"github.com/kedacore/http-add-on/pkg/routing"
)

type forwardingConfig struct {
	waitTimeout           time.Duration
	respHeaderTimeout     time.Duration
	forceAttemptHTTP2     bool
	maxIdleConns          int
	idleConnTimeout       time.Duration
	tlsHandshakeTimeout   time.Duration
	expectContinueTimeout time.Duration
}

func newForwardingConfigFromTimeouts(t *config.Timeouts) forwardingConfig {
	return forwardingConfig{
		waitTimeout:           t.DeploymentReplicas,
		respHeaderTimeout:     t.ResponseHeader,
		forceAttemptHTTP2:     t.ForceHTTP2,
		maxIdleConns:          t.MaxIdleConns,
		idleConnTimeout:       t.IdleConnTimeout,
		tlsHandshakeTimeout:   t.TLSHandshakeTimeout,
		expectContinueTimeout: t.ExpectContinueTimeout,
	}
}

// newForwardingHandler takes in the service URL for the app backend
// and forwards incoming requests to it. Note that it isn't multitenant.
// It's intended to be deployed and scaled alongside the application itself.
//
// fwdSvcURL must have a valid scheme in it. The best way to do this is
// create a URL with url.Parse("https://...")
func newForwardingHandler(
	lggr logr.Logger,
	routingTable *routing.Table,
	dialCtxFunc kedanet.DialContextFunc,
	waitFunc forwardWaitFunc,
	fwdCfg forwardingConfig,
) http.Handler {
	roundTripper := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           dialCtxFunc,
		ForceAttemptHTTP2:     fwdCfg.forceAttemptHTTP2,
		MaxIdleConns:          fwdCfg.maxIdleConns,
		IdleConnTimeout:       fwdCfg.idleConnTimeout,
		TLSHandshakeTimeout:   fwdCfg.tlsHandshakeTimeout,
		ExpectContinueTimeout: fwdCfg.expectContinueTimeout,
		ResponseHeaderTimeout: fwdCfg.respHeaderTimeout,
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, err := getHost(r)
		if err != nil {
			w.WriteHeader(400)
			w.Write([]byte("Host not found in request"))
			return
		}
		routingTarget, err := routingTable.Lookup(host)
		if err != nil {
			w.WriteHeader(404)
			w.Write([]byte(fmt.Sprintf("Host %s not found", r.Host)))
			return
		}

		ctx, done := context.WithTimeout(r.Context(), fwdCfg.waitTimeout)
		defer done()
		if err := waitFunc(ctx, routingTarget.Deployment); err != nil {
			lggr.Error(err, "wait function failed, not forwarding request")
			w.WriteHeader(502)
			w.Write([]byte(fmt.Sprintf("error on backend (%s)", err)))
			return
		}
		targetSvcURL, err := routingTarget.ServiceURL()
		if err != nil {
			lggr.Error(err, "forwarding failed")
			w.WriteHeader(500)
			w.Write([]byte("error getting backend service URL"))
			return
		}
		forwardRequest(w, r, roundTripper, targetSvcURL)
	})
}
