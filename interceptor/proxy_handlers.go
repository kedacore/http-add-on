package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"

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
	fwdSvcURL *url.URL,
	dialCtxFunc kedanet.DialContextFunc,
	waitFunc forwardWaitFunc,
	waitTimeout time.Duration,
	respTimeout time.Duration,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, done := context.WithTimeout(r.Context(), waitTimeout)
		defer done()
		grp, _ := errgroup.WithContext(ctx)
		grp.Go(waitFunc)
		waitErr := grp.Wait()
		if waitErr != nil {
			log.Printf("Error, not forwarding request")
			w.WriteHeader(502)
			w.Write([]byte(fmt.Sprintf("error on backend (%s)", waitErr)))
			return
		}

		forwardRequest(w, r, dialCtxFunc, respTimeout, fwdSvcURL)
	})
}
