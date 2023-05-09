package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"

	"github.com/kedacore/http-add-on/pkg/k8s"
	kedanet "github.com/kedacore/http-add-on/pkg/net"
	"github.com/kedacore/http-add-on/pkg/queue"
)

func TestCounts(t *testing.T) {
	r := require.New(t)
	ctx := context.Background()
	const (
		ns           = "testns"
		svcName      = "testsvc"
		deplName     = "testdepl"
		tickDur      = 10 * time.Millisecond
		numEndpoints = 3
	)

	// assemble an in-memory queue and start up a fake server that serves it.
	// we'll configure the queue pinger to use that server below
	counts := map[string]int{
		"host1": 123,
		"host2": 234,
		"host3": 456,
		"host4": 809,
	}

	q := queue.NewMemory()
	for host, count := range counts {
		r.NoError(q.Resize(host, count))
	}

	srv, srvURL, endpoints, err := startFakeQueueEndpointServer(
		svcName,
		q,
		3,
	)
	r.NoError(err)
	defer srv.Close()
	pinger, err := newQueuePinger(
		ctx,
		logr.Discard(),
		func(context.Context, string, string) (*v1.Endpoints, error) {
			return endpoints, nil
		},
		ns,
		svcName,
		deplName,
		srvURL.Port(),
	)
	r.NoError(err)
	// the pinger does an initial fetch, so ensure that
	// the saved counts are correct
	retCounts := pinger.counts()
	r.Equal(len(counts), len(retCounts))

	// now update the queue, start the ticker, and ensure
	// that counts are updated after the first tick
	r.NoError(q.Resize("host1", 1))
	r.NoError(q.Resize("host2", 2))
	r.NoError(q.Resize("host3", 3))
	r.NoError(q.Resize("host4", 4))
	ticker := time.NewTicker(tickDur)
	fakeCache := k8s.NewFakeDeploymentCache()
	go func() {
		_ = pinger.start(ctx, ticker, fakeCache)
	}()
	// sleep to ensure we ticked and finished calling
	// fetchAndSaveCounts
	time.Sleep(tickDur * 2)

	// now ensure that all the counts in the pinger
	// are the same as in the queue, which has been updated
	retCounts = pinger.counts()
	expectedCounts, err := q.Current()
	r.NoError(err)
	r.Equal(len(expectedCounts.Counts), len(retCounts))
	for host, count := range expectedCounts.Counts {
		retCount, ok := retCounts[host]
		r.True(
			ok,
			"returned count not found for host %s",
			host,
		)

		// note that the returned value should be:
		// (queue_count * num_endpoints)
		r.Equal(count*3, retCount)
	}
}

func TestFetchAndSaveCounts(t *testing.T) {
	r := require.New(t)
	ctx, done := context.WithCancel(context.Background())
	defer done()
	const (
		ns           = "testns"
		svcName      = "testsvc"
		deplName     = "testdepl"
		adminPort    = "8081"
		numEndpoints = 3
	)
	counts := queue.NewCounts()
	counts.Counts = map[string]int{
		"host1": 123,
		"host2": 234,
		"host3": 345,
	}
	q := queue.NewMemory()
	for host, count := range counts.Counts {
		r.NoError(q.Resize(host, count))
	}
	srv, srvURL, endpoints, err := startFakeQueueEndpointServer(
		svcName, q, numEndpoints,
	)
	r.NoError(err)
	defer srv.Close()
	endpointsFn := func(
		ctx context.Context,
		ns,
		svcName string,
	) (*v1.Endpoints, error) {
		return endpoints, nil
	}

	pinger, err := newQueuePinger(
		ctx,
		logr.Discard(),
		endpointsFn,
		ns,
		svcName,
		deplName,
		srvURL.Port(),
		// time.NewTicker(1*time.Millisecond),
	)
	r.NoError(err)

	r.NoError(pinger.fetchAndSaveCounts(ctx))

	// since all endpoints serve the same counts,
	// expected aggregate is individual count * # endpoints
	expectedAgg := counts.Aggregate() * numEndpoints
	r.Equal(expectedAgg, pinger.aggregateCount)
	// again, since all endpoints serve the same counts,
	// the hosts will be the same as the original counts,
	// but the value is (individual count * # endpoints)
	expectedCounts := counts.Counts
	for host, val := range expectedCounts {
		expectedCounts[host] = val * numEndpoints
	}
	r.Equal(expectedCounts, pinger.allCounts)
}

func TestFetchCounts(t *testing.T) {
	r := require.New(t)
	ctx, done := context.WithCancel(context.Background())
	defer done()
	const (
		ns           = "testns"
		svcName      = "testsvc"
		adminPort    = "8081"
		numEndpoints = 3
	)
	counts := queue.NewCounts()
	counts.Counts = map[string]int{
		"host1": 123,
		"host2": 234,
		"host3": 345,
	}
	q := queue.NewMemory()
	for host, count := range counts.Counts {
		r.NoError(q.Resize(host, count))
	}
	srv, srvURL, endpoints, err := startFakeQueueEndpointServer(
		svcName, q, numEndpoints,
	)
	r.NoError(err)

	defer srv.Close()
	endpointsFn := func(
		context.Context,
		string,
		string,
	) (*v1.Endpoints, error) {
		return endpoints, nil
	}

	cts, agg, err := fetchCounts(
		ctx,
		logr.Discard(),
		endpointsFn,
		ns,
		svcName,
		srvURL.Port(),
	)
	r.NoError(err)
	// since all endpoints serve the same counts,
	// expected aggregate is individual count * # endpoints
	expectedAgg := counts.Aggregate() * numEndpoints
	r.Equal(expectedAgg, agg)
	// again, since all endpoints serve the same counts,
	// the hosts will be the same as the original counts,
	// but the value is (individual count * # endpoints)
	expectedCounts := counts.Counts
	for host, val := range expectedCounts {
		expectedCounts[host] = val * numEndpoints
	}
	r.Equal(expectedCounts, cts)
}

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
	svcName string,
	q queue.CountReader,
	numEndpoints int,
) (*httptest.Server, *url.URL, *v1.Endpoints, error) {
	hdl := http.NewServeMux()
	queue.AddCountsRoute(logr.Discard(), hdl, q)
	srv, srvURL, err := kedanet.StartTestServer(hdl)
	if err != nil {
		return nil, nil, nil, err
	}
	endpoints, err := k8s.FakeEndpointsForURL(srvURL, "testns", svcName, numEndpoints)
	if err != nil {
		return nil, nil, nil, err
	}
	return srv, srvURL, endpoints, nil
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
) (*time.Ticker, *queuePinger, error) {
	opts := &fakeQueuePingerOpts{
		endpoints: &v1.Endpoints{},
		tickDur:   time.Second,
		port:      "8080",
	}
	for _, optsFunc := range optsFuncs {
		optsFunc(opts)
	}
	ticker := time.NewTicker(opts.tickDur)

	pinger, err := newQueuePinger(
		ctx,
		lggr,
		func(context.Context, string, string) (*v1.Endpoints, error) {
			return opts.endpoints, nil
		},
		"testns",
		"testsvc",
		"testdepl",
		opts.port,
	)
	if err != nil {
		return nil, nil, err
	}
	return ticker, pinger, nil
}
