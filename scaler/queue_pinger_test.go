package main

import (
	context "context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/kedacore/http-add-on/pkg/k8s"
	kedanet "github.com/kedacore/http-add-on/pkg/net"
	"github.com/kedacore/http-add-on/pkg/queue"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
)

func TestCounts(t *testing.T) {
	r := require.New(t)
	ctx := context.Background()
	const (
		ns      = "testns"
		svcName = "testsvc"
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
		q.Resize(host, count)
	}

	hdl := http.NewServeMux()
	queue.AddCountsRoute(logr.Discard(), hdl, q)
	srv, srvURL, err := kedanet.StartTestServer(hdl)
	r.NoError(err)
	defer srv.Close()

	endpoints, err := k8s.FakeEndpointsForURL(srvURL, ns, svcName, 3)
	r.NoError(err)
	// set the initial ticker to effectively never tick so that we
	// can check the behavior of the pinger before the first
	// tick
	ticker := time.NewTicker(10000 * time.Hour)
	pinger := newQueuePinger(
		ctx,
		logr.Discard(),
		func(context.Context, string, string) (*v1.Endpoints, error) {
			return endpoints, nil
		},
		ns,
		svcName,
		srvURL.Port(),
		ticker,
	)
	// the pinger starts a background watch loop but won't request the counts
	// before the first tick. since the first tick effectively won't
	// happen (it was set to a very long duration above), there should be
	// no counts right now
	retCounts := pinger.counts()
	r.Equal(0, len(retCounts))

	// reset the ticker to tick practically immediately. sleep for a little
	// bit to ensure that the tick occurred and the counts were successfully
	// computed, then check them.
	ticker.Reset(1 * time.Nanosecond)
	time.Sleep(50 * time.Millisecond)

	// now that the tick has happened, there should be as many
	// key/value pairs in the returned counts map as addresses
	retCounts = pinger.counts()
	r.Equal(len(counts), len(retCounts))

	// each interceptor returns the same counts, so for each host in
	// the counts map, the integer count should be
	// (val * # interceptors)
	for retHost, retCount := range retCounts {
		expectedCount, ok := counts[retHost]
		r.True(ok, "unexpected host %s returned", retHost)
		expectedCount *= len(endpoints.Subsets[0].Addresses)
		r.Equal(
			expectedCount,
			retCount,
			"count for host %s was not the expected %d",
			retCount,
			expectedCount,
		)
	}

}

func TestFetchAndSaveCounts(t *testing.T) {
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
	hdl := kedanet.NewTestHTTPHandlerWrapper(
		http.HandlerFunc(
			func(wr http.ResponseWriter, req *http.Request) {
				err := json.NewEncoder(wr).Encode(counts)
				r.NoError(err)
			},
		),
	)
	srv, srvURL, err := kedanet.StartTestServer(hdl)
	r.NoError(err)
	endpointsForURLs, err := k8s.FakeEndpointsForURL(
		srvURL,
		ns,
		svcName,
		numEndpoints,
	)
	r.NoError(err)
	defer srv.Close()
	endpointsFn := func(
		ctx context.Context,
		ns,
		svcName string,
	) (*v1.Endpoints, error) {
		return endpointsForURLs, nil
	}

	pinger := newQueuePinger(
		ctx,
		logr.Discard(),
		endpointsFn,
		ns,
		svcName,
		srvURL.Port(),
		time.NewTicker(1*time.Millisecond),
	)

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
	hdl := kedanet.NewTestHTTPHandlerWrapper(
		http.HandlerFunc(
			func(wr http.ResponseWriter, req *http.Request) {
				err := json.NewEncoder(wr).Encode(counts)
				r.NoError(err)
			},
		),
	)
	srv, srvURL, err := kedanet.StartTestServer(hdl)
	r.NoError(err)
	endpointsForURLs, err := k8s.FakeEndpointsForURL(
		srvURL,
		ns,
		svcName,
		numEndpoints,
	)
	r.NoError(err)
	defer srv.Close()
	endpointsFn := func(
		ctx context.Context,
		ns,
		svcName string,
	) (*v1.Endpoints, error) {
		return endpointsForURLs, nil
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
