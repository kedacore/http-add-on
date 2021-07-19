package main

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/go-logr/logr"
	kedanet "github.com/kedacore/http-add-on/pkg/net"
	"golang.org/x/sync/errgroup"
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
	logger logr.Logger,
	fwdSvcURL *url.URL,
	dialCtxFunc kedanet.DialContextFunc,
	waitFunc forwardWaitFunc,
	waitTimeout time.Duration,
	respHeaderTimeout time.Duration,
) http.Handler {
	logger = logger.WithName("newForwardingHandler")
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
		ctx, done := context.WithTimeout(r.Context(), waitTimeout)
		defer done()
		grp, _ := errgroup.WithContext(ctx)
		grp.Go(waitFunc)
		waitErr := grp.Wait()
		if waitErr != nil {
			logger.Error(waitErr, "Returning 502 to client")
			w.WriteHeader(502)
			w.Write([]byte(fmt.Sprintf("error on backend (%s)", waitErr)))
			return
		}

		forwardRequest(w, r, roundTripper, fwdSvcURL)
	})
}
