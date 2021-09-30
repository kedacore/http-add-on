package main

import (
	context "context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"time"

	"github.com/go-logr/logr"
	"github.com/kedacore/http-add-on/pkg/k8s"
	kedanet "github.com/kedacore/http-add-on/pkg/net"
	"github.com/kedacore/http-add-on/pkg/queue"
	v1 "k8s.io/api/core/v1"
)

// startFakeQueuePinger starts a fake server that simulates
// an interceptor with its /queue endpoint, then returns a
// *v1.Endpoints object that contains the URL of the new fake
// server. also returns the *httptest.Server that runs the
// endpoint along with its URL. the caller is responsible for
// calling testServer.Close() when done.
//
// returns nil for the first 3 return value and a non-nil error in
// case of a failure.
func startFakeQueueEndpointServer(
	ns,
	svcName string,
	q queue.CountReader,
	numEndpoints int,
) (*httptest.Server, *url.URL, *v1.Endpoints, error) {
	hdl := http.NewServeMux()
	queue.AddCountsRoute(logr.Discard(), hdl, q)
	srv, url, err := kedanet.NewTestServer(hdl)
	if err != nil {
		return nil, nil, nil, err
	}

	endpoints := k8s.FakeEndpointsForURL(url, ns, svcName, numEndpoints)
	return srv, url, endpoints, nil
}

type fakeQueuePingerOpts struct {
	endpoints *v1.Endpoints
	tickDur   time.Duration
	port      string
}

type optsFunc func(*fakeQueuePingerOpts)

// newFakeQueuePinger creates the machinery required for a fake
// queuePinger implementation, including a time.Ticker, then returns
// the ticker and the pinger. it is the caller's responsibility to
// call ticker.Stop() on the returned ticker.
func newFakeQueuePinger(
	ctx context.Context,
	lggr logr.Logger,
	optsFuncs ...optsFunc,
) (*time.Ticker, *queuePinger) {
	opts := &fakeQueuePingerOpts{
		endpoints: &v1.Endpoints{},
		tickDur:   time.Second,
		port:      "8080",
	}
	for _, optsFunc := range optsFuncs {
		optsFunc(opts)
	}
	ticker := time.NewTicker(opts.tickDur)
	pinger := newQueuePinger(
		ctx,
		lggr,
		func(
			ctx context.Context,
			namespace,
			serviceName string,
		) (*v1.Endpoints, error) {
			return opts.endpoints, nil

		},
		"testns",
		"testsvc",
		opts.port,
		ticker,
	)
	return ticker, pinger
}
