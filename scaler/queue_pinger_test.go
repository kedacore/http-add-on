package main

import (
	context "context"
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

func TestRequestCounts(t *testing.T) {
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
	srv, url, err := kedanet.StartTestServer(hdl)
	r.NoError(err)
	defer srv.Close()

	endpoints := k8s.FakeEndpointsForURL(url, ns, svcName, 3)
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
		url.Port(),
		ticker,
	)
	// the pinger starts a background watch loop but won't request the counts
	// before the first tick. since the first tick effectively won't
	// happen (it was set to a very long duration above), there should be
	// no counts right now
	retCounts := pinger.counts()
	r.Equal(0, len(retCounts))

	// reset the ticker to tick practically immediatly. sleep for a little
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
