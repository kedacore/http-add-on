package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/go-logr/logr"
	kedanet "github.com/kedacore/http-add-on/pkg/net"
	"github.com/kedacore/http-add-on/pkg/routing"
)

func moreThanPtr(i *int32, target int32) bool {
	return i != nil && *i > target
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
	waitTimeout time.Duration,
	respHeaderTimeout time.Duration,
) http.Handler {
	roundTripper := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           dialCtxFunc,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ResponseHeaderTimeout: respHeaderTimeout,
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, err := getHost(r)
		if err != nil {
			w.WriteHeader(400)
			w.Write([]byte("Host not found in request"))
			return
		}
		fmt.Println("GOT HOST", host)
		fmt.Println("ABOUT TO LOOK UP HOST IN ROUTING TABLE", routingTable)
		routingTarget, err := routingTable.Lookup(host)
		if err != nil {
			w.WriteHeader(404)
			w.Write([]byte(fmt.Sprintf("Host %s not found", r.Host)))
			return
		}
		fmt.Println("GOT ROUTING TARGET", routingTarget)
		ctx, done := context.WithTimeout(r.Context(), waitTimeout)
		defer done()
		if err := waitFunc(ctx, routingTarget.Deployment); err != nil {
			lggr.Error(err, "wait function failed, not forwarding request")
			w.WriteHeader(502)
			w.Write([]byte(fmt.Sprintf("error on backend (%s)", err)))
			return
		}
		fmt.Println("DONE WAITING")
		targetSvcURL, err := routingTarget.ServiceURL()
		if err != nil {
			lggr.Error(err, "forwarding failed")
			w.WriteHeader(500)
			w.Write([]byte("error getting backend service URL"))
			return
		}
		fmt.Println("ABOUT TO REQUEST TO SERVICE", *targetSvcURL)
		forwardRequest(w, r, roundTripper, targetSvcURL)
		fmt.Println("DONE")
	})
}
