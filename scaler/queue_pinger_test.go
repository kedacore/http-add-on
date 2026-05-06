package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/require"

	"github.com/kedacore/http-add-on/pkg/k8s"
	kedanet "github.com/kedacore/http-add-on/pkg/net"
	"github.com/kedacore/http-add-on/pkg/queue"
)

func TestCounts(t *testing.T) {
	r := require.New(t)
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	const (
		ns       = "testns"
		svcName  = "testsvc"
		deplName = "testdepl"
		tickDur  = 10 * time.Millisecond
	)

	q := queue.NewMemory()
	hosts := map[string]int{
		"host1": 123,
		"host2": 234,
		"host3": 345,
		"host4": 456,
	}
	for host, n := range hosts {
		q.EnsureKey(host)
		r.NoError(q.Increase(host, n))
	}

	srv, srvURL, endpoints, err := startFakeQueueEndpointServer(q)
	r.NoError(err)
	defer srv.Close()
	pinger := newQueuePinger(
		logr.Discard(),
		func(context.Context, string, string) (k8s.Endpoints, error) {
			return endpoints, nil
		},
		ns,
		svcName,
		deplName,
		srvURL.Port(),
	)

	ticker := time.NewTicker(tickDur)
	go func() {
		_ = pinger.start(ctx, ticker)
	}()
	time.Sleep(tickDur * 3)

	retCounts := pinger.counts()
	r.Len(retCounts, len(hosts))
	for host, n := range hosts {
		retCount, ok := retCounts[host]
		r.True(ok, "returned count not found for host %s", host)
		r.Equal(n, retCount.Concurrency)
	}
}

func TestFetchAndSaveCounts(t *testing.T) {
	r := require.New(t)
	ctx := t.Context()
	const (
		ns       = "testns"
		svcName  = "testsvc"
		deplName = "testdepl"
	)

	q := queue.NewMemory()
	hosts := map[string]int{
		"host1": 123,
		"host2": 234,
		"host3": 345,
		"host4": 456,
	}
	for host, n := range hosts {
		q.EnsureKey(host)
		r.NoError(q.Increase(host, n))
	}
	srv, srvURL, endpoints, err := startFakeQueueEndpointServer(q)
	r.NoError(err)
	defer srv.Close()
	endpointsFn := func(ctx context.Context, ns, svcName string) (k8s.Endpoints, error) {
		return endpoints, nil
	}

	pinger := newQueuePinger(
		logr.Discard(),
		endpointsFn,
		ns,
		svcName,
		deplName,
		srvURL.Port(),
	)

	r.NoError(pinger.fetchAndSaveCounts(ctx))

	for host, n := range hosts {
		count, ok := pinger.allCounts[host]
		r.True(ok, "host %s missing", host)
		r.Equal(n, count.Concurrency)
		// First fetch: no previous data, so RequestRate should be 0.
		r.InDelta(0.0, count.RequestRate, 0.001)
	}
}

func TestFetchCountsPerPod(t *testing.T) {
	r := require.New(t)
	ctx := t.Context()
	const (
		ns      = "testns"
		svcName = "testsvc"
	)

	q := queue.NewMemory()
	hosts := map[string]int{
		"host1": 123,
		"host2": 234,
	}
	for host, n := range hosts {
		q.EnsureKey(host)
		r.NoError(q.Increase(host, n))
	}
	srv, srvURL, endpoints, err := startFakeQueueEndpointServer(q)
	r.NoError(err)
	defer srv.Close()
	endpointsFn := func(context.Context, string, string) (k8s.Endpoints, error) {
		return endpoints, nil
	}

	perPod, err := fetchCountsPerPod(
		ctx,
		logr.Discard(),
		endpointsFn,
		ns,
		svcName,
		fmt.Sprintf("%v", srvURL.Port()),
	)
	r.NoError(err)
	r.Len(perPod, 1)

	for _, counts := range perPod {
		for host, n := range hosts {
			c, ok := counts[host]
			r.True(ok, "host %s missing from pod result", host)
			r.Equal(n, c.Concurrency)
			r.Equal(int64(n), c.RequestCount)
		}
	}
}

func TestFetchAndSaveCounts_MultiPodLifecycle(t *testing.T) {
	r := require.New(t)
	ctx := t.Context()
	const (
		ns        = "testns"
		svcName   = "testsvc"
		deplName  = "testdepl"
		adminPort = "8081"
	)

	var podAReq atomic.Int64
	podAReq.Store(100)
	var podBReq atomic.Int64
	podBReq.Store(1000)

	podA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(queue.Counts{
			"host1": {
				Concurrency:  2,
				RequestCount: podAReq.Load(),
			},
		})
	}))
	defer podA.Close()

	podB := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(queue.Counts{
			"host1": {
				Concurrency:  3,
				RequestCount: podBReq.Load(),
			},
		})
	}))
	defer podB.Close()

	withPatchedDefaultTransport(t, map[string]string{
		"pod-a:" + adminPort: podA.Listener.Addr().String(),
		"pod-b:" + adminPort: podB.Listener.Addr().String(),
	})

	var readyAddrs atomic.Value
	readyAddrs.Store([]string{"pod-a"})

	pinger := newQueuePinger(
		logr.Discard(),
		func(context.Context, string, string) (k8s.Endpoints, error) {
			addrs := readyAddrs.Load().([]string)
			return k8s.Endpoints{
				ReadyAddresses: append([]string(nil), addrs...),
			}, nil
		},
		ns,
		svcName,
		deplName,
		adminPort,
	)

	// Baseline with one pod.
	r.NoError(pinger.fetchAndSaveCounts(ctx))
	count := pinger.count("host1")
	r.Equal(2, count.Concurrency)
	r.InDelta(0.0, count.RequestRate, 0.001)

	// Add a new pod with a high existing counter:
	// this should not spike rate immediately.
	readyAddrs.Store([]string{"pod-a", "pod-b"})
	r.NoError(pinger.fetchAndSaveCounts(ctx))
	count = pinger.count("host1")
	r.Equal(5, count.Concurrency)
	r.InDelta(0.0, count.RequestRate, 0.001)

	// Next tick after both pods increase should produce non-zero rate.
	podAReq.Store(130)  // +30
	podBReq.Store(1060) // +60
	r.NoError(pinger.fetchAndSaveCounts(ctx))
	count = pinger.count("host1")
	r.Equal(5, count.Concurrency)
	r.Greater(count.RequestRate, 0.0)

	// Remove pod-b and ensure its previous counters are pruned.
	readyAddrs.Store([]string{"pod-a"})
	podAReq.Store(150) // +20
	r.NoError(pinger.fetchAndSaveCounts(ctx))
	count = pinger.count("host1")
	r.Equal(2, count.Concurrency)
	r.NotContains(pinger.prevPodCounts, "pod-b:"+adminPort)
}

func TestRateComputation(t *testing.T) {
	r := require.New(t)
	ctx := t.Context()
	const (
		ns       = "testns"
		svcName  = "testsvc"
		deplName = "testdepl"
	)

	// Use a dynamic server whose RequestCount increases between polls.
	var reqCount atomic.Int64
	reqCount.Store(100)

	hdl := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		cur := reqCount.Load()
		counts := queue.Counts{
			"host1": {Concurrency: 5, RequestCount: cur},
		}
		_ = json.NewEncoder(w).Encode(counts)
	})

	srv := httptest.NewServer(hdl)
	defer srv.Close()
	srvURL, _ := url.Parse(srv.URL)

	pinger := newQueuePinger(
		logr.Discard(),
		func(context.Context, string, string) (k8s.Endpoints, error) {
			return k8s.Endpoints{ReadyAddresses: []string{srvURL.Hostname()}}, nil
		},
		ns,
		svcName,
		deplName,
		srvURL.Port(),
	)

	// First poll: establishes baseline (no delta, RequestRate=0)
	r.NoError(pinger.fetchAndSaveCounts(ctx))
	r.Equal(5, pinger.allCounts["host1"].Concurrency)
	r.InDelta(0.0, pinger.allCounts["host1"].RequestRate, 0.001)

	// Simulate 50 new requests arriving.
	reqCount.Store(150)

	// Second poll: delta = 150 - 100 = 50
	r.NoError(pinger.fetchAndSaveCounts(ctx))
	r.Equal(5, pinger.allCounts["host1"].Concurrency)
	r.Greater(pinger.allCounts["host1"].RequestRate, 0.0)
}

func TestRateComputationCounterReset(t *testing.T) {
	r := require.New(t)
	ctx := t.Context()
	const (
		ns       = "testns"
		svcName  = "testsvc"
		deplName = "testdepl"
	)

	var reqCount atomic.Int64
	reqCount.Store(500)

	hdl := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		cur := reqCount.Load()
		counts := queue.Counts{
			"host1": {Concurrency: 1, RequestCount: cur},
		}
		_ = json.NewEncoder(w).Encode(counts)
	})

	srv := httptest.NewServer(hdl)
	defer srv.Close()
	srvURL, _ := url.Parse(srv.URL)

	pinger := newQueuePinger(
		logr.Discard(),
		func(context.Context, string, string) (k8s.Endpoints, error) {
			return k8s.Endpoints{ReadyAddresses: []string{srvURL.Hostname()}}, nil
		},
		ns,
		svcName,
		deplName,
		srvURL.Port(),
	)

	// First poll: baseline
	r.NoError(pinger.fetchAndSaveCounts(ctx))

	// Simulate pod restart: counter resets to a small value.
	reqCount.Store(10)

	// Second poll: counter went down (500 → 10), treated as reset.
	// delta = current value (10), not negative.
	r.NoError(pinger.fetchAndSaveCounts(ctx))
	r.Greater(pinger.allCounts["host1"].RequestRate, 0.0)
}

func TestUpdateBucketConfig(t *testing.T) {
	r := require.New(t)
	_, pinger, err := newFakeQueuePinger(logr.Discard())
	r.NoError(err)

	pinger.UpdateBucketConfig("host1", 2*time.Minute, 2*time.Second)

	b, ok := pinger.rateBuckets["host1"]
	r.True(ok)
	r.Equal(2*time.Minute, b.Window())
	r.Equal(2*time.Second, b.Granularity())
}

// startFakeQueueEndpointServer starts a fake server that simulates
// an interceptor with its /queue endpoint. Returns the test server,
// its URL, and a k8s.Endpoints pointing at it. The caller is
// responsible for calling testServer.Close() when done.
func startFakeQueueEndpointServer(q queue.CountReader) (*httptest.Server, *url.URL, k8s.Endpoints, error) {
	hdl := http.NewServeMux()
	queue.AddCountsRoute(logr.Discard(), hdl, q)
	srv, srvURL, err := kedanet.StartTestServer(hdl)
	if err != nil {
		return nil, nil, k8s.Endpoints{}, err
	}
	return srv, srvURL, k8s.Endpoints{ReadyAddresses: []string{srvURL.Hostname()}}, nil
}

type fakeQueuePingerOpts struct {
	endpoints k8s.Endpoints
	tickDur   time.Duration
	port      string
}

type optsFunc func(*fakeQueuePingerOpts)

// newFakeQueuePinger creates the machinery required for a fake
// queuePinger implementation, including a time.Ticker, then returns
// the ticker and the pinger. it is the caller's responsibility to
// call ticker.Stop() on the returned ticker.
func newFakeQueuePinger(lggr logr.Logger, optsFuncs ...optsFunc) (*time.Ticker, *queuePinger, error) {
	opts := &fakeQueuePingerOpts{
		tickDur: time.Second,
		port:    "8080",
	}
	for _, optsFunc := range optsFuncs {
		optsFunc(opts)
	}
	ticker := time.NewTicker(opts.tickDur)

	pinger := newQueuePinger(
		lggr,
		func(context.Context, string, string) (k8s.Endpoints, error) {
			return opts.endpoints, nil
		},
		"testns",
		"testsvc",
		"testdepl",
		opts.port,
	)
	return ticker, pinger, nil
}

func withPatchedDefaultTransport(t *testing.T, dialMap map[string]string) {
	t.Helper()

	oldTransport := http.DefaultTransport
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			if mappedAddr, ok := dialMap[addr]; ok {
				addr = mappedAddr
			}
			return (&net.Dialer{}).DialContext(ctx, network, addr)
		},
	}
	http.DefaultTransport = transport

	t.Cleanup(func() {
		http.DefaultTransport = oldTransport
		transport.CloseIdleConnections()
	})
}
