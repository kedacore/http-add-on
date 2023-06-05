package main

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
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
	routingTable routing.Table,
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
		routingTarget := routingTable.Route(r)
		if routingTarget == nil {
			w.WriteHeader(http.StatusNotFound)
			if _, err := w.Write([]byte(fmt.Sprintf("Host %s not found", r.Host))); err != nil {
				lggr.Error(err, "could not send error message to client")
			}

			return
		}

		waitFuncCtx, done := context.WithTimeout(r.Context(), fwdCfg.waitTimeout)
		defer done()
		replicas, err := waitFunc(
			waitFuncCtx,
			routingTarget.Namespace,
			routingTarget.Spec.ScaleTargetRef.Deployment,
		)
		if err != nil {
			lggr.Error(err, "wait function failed, not forwarding request")
			w.WriteHeader(http.StatusBadGateway)
			if _, err := w.Write([]byte(fmt.Sprintf("error on backend (%s)", err))); err != nil {
				lggr.Error(err, "could not write error response to client")
			}
			return
		}
		//goland:noinspection HttpUrlsUsage
		targetSvcURL, err := url.Parse(fmt.Sprintf(
			"http://%s.%s:%d",
			routingTarget.Spec.ScaleTargetRef.Service,
			routingTarget.Namespace,
			routingTarget.Spec.ScaleTargetRef.Port,
		))
		if err != nil {
			lggr.Error(err, "forwarding failed")
			w.WriteHeader(http.StatusInternalServerError)
			if _, err := w.Write([]byte("error getting backend service URL")); err != nil {
				lggr.Error(err, "could not write error response to client")
			}
			return
		}
		isColdStart := "false"
		if replicas == 0 {
			isColdStart = "true"
		}
		w.Header().Add("X-KEDA-HTTP-Cold-Start", isColdStart)
		forwardRequest(lggr, w, r, roundTripper, targetSvcURL)
	})
}
