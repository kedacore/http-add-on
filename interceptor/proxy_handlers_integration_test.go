package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/kedacore/http-add-on/interceptor/config"
	"github.com/kedacore/http-add-on/interceptor/middleware"
	"github.com/kedacore/http-add-on/pkg/k8s"
	kedanet "github.com/kedacore/http-add-on/pkg/net"
	routingtest "github.com/kedacore/http-add-on/pkg/routing/test"
)

// happy path - deployment is scaled to 1 and host in routing table
func TestIntegrationHappyPath(t *testing.T) {
	const (
		activeEndpointsTimeout = 200 * time.Millisecond
		deploymentName         = "testdeployment"
		serviceName            = "testservice"
	)
	r := require.New(t)
	h, err := newHarness(
		t,
		activeEndpointsTimeout,
	)
	r.NoError(err)
	defer h.close()
	t.Logf("Harness: %s", h.String())

	originPort, err := strconv.Atoi(h.originURL.Port())
	r.NoError(err)

	target := targetFromURL(
		h.originURL,
		originPort,
		deploymentName,
		serviceName,
	)
	h.routingTable.Memory[hostForTest(t)] = target

	h.endpCache.Set(v1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName,
			Namespace: target.GetNamespace(),
		},
	})
	r.NoError(h.endpCache.SetSubsets(target.GetNamespace(), serviceName, 1))

	// happy path
	res, err := doRequest(
		http.DefaultClient,
		h.proxyURL.String(),
		hostForTest(t),
	)
	r.NoError(err)
	r.Equal(200, res.StatusCode)
	res.Body.Close()
}

// deployment scaled to 1 but host not in routing table
//
// NOTE: the interceptor needs to check in the routing table
// _before_ checking the endpoints cache, so we don't technically
// need to set the replicas to 1, but we're doing so anyway to
// isolate the routing table behavior
func TestIntegrationNoRoutingTableEntry(t *testing.T) {
	r := require.New(t)
	h, err := newHarness(t, time.Second)
	r.NoError(err)
	defer h.close()

	// not in the routing table
	res, err := doRequest(
		http.DefaultClient,
		h.proxyURL.String(),
		"not-in-the-table",
	)
	res.Body.Close()
	r.NoError(err)
	r.Equal(404, res.StatusCode)
	res.Body.Close()
}

// host in the routing table but deployment has no replicas
func TestIntegrationNoReplicas(t *testing.T) {
	const (
		activeEndpointsTimeout = 100 * time.Millisecond
	)
	host := hostForTest(t)
	deploymentName := "testdeployment"
	serviceName := "testservice"
	r := require.New(t)
	h, err := newHarness(t, activeEndpointsTimeout)
	r.NoError(err)

	originPort, err := strconv.Atoi(h.originURL.Port())
	r.NoError(err)

	target := targetFromURL(
		h.originURL,
		originPort,
		deploymentName,
		serviceName,
	)
	h.routingTable.Memory[hostForTest(t)] = target

	// 0 replicas
	h.endpCache.Set(v1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName,
			Namespace: target.GetNamespace(),
		},
	})

	start := time.Now()
	res, err := doRequest(
		http.DefaultClient,
		h.proxyURL.String(),
		host,
	)
	r.NoError(err)
	r.Equal(502, res.StatusCode)
	res.Body.Close()
	elapsed := time.Since(start)
	// we should have slept more than the active endpoints wait timeout
	r.GreaterOrEqual(elapsed, activeEndpointsTimeout)
	r.Less(elapsed, activeEndpointsTimeout+50*time.Millisecond)
}

// the request comes in while there are no replicas, and one is added
// while it's pending
func TestIntegrationWaitReplicas(t *testing.T) {
	const (
		activeEndpointsTimeout = 2 * time.Second
		responseTimeout        = 1 * time.Second
		deploymentName         = "testdeployment"
		serviceName            = "testservice"
	)
	ctx := context.Background()
	r := require.New(t)
	h, err := newHarness(t, activeEndpointsTimeout)
	r.NoError(err)

	// add host to routing table
	originPort, err := strconv.Atoi(h.originURL.Port())
	r.NoError(err)

	target := targetFromURL(
		h.originURL,
		originPort,
		deploymentName,
		serviceName,
	)
	h.routingTable.Memory[hostForTest(t)] = target

	// set up a endpoint with zero replicas and create
	// a watcher we can use later to fake-send a endpoint
	// event
	h.endpCache.Set(v1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName,
			Namespace: target.GetNamespace(),
		},
	})
	endpoints, _ := h.endpCache.Get(target.GetNamespace(), serviceName)
	watcher := h.endpCache.SetWatcher(target.GetNamespace(), serviceName)

	// make the request in one goroutine, and in the other, wait a bit
	// and then add replicas to the endpoints cache

	var response *http.Response
	grp, _ := errgroup.WithContext(ctx)
	grp.Go(func() error {
		resp, err := doRequest(
			http.DefaultClient,
			h.proxyURL.String(),
			hostForTest(t),
		)
		if err != nil {
			return err
		}
		response = resp
		resp.Body.Close()
		return nil
	})
	const sleepDur = activeEndpointsTimeout / 4
	grp.Go(func() error {
		t.Logf("Sleeping for %s", sleepDur)
		time.Sleep(sleepDur)
		t.Logf("Woke up, setting replicas to 10")

		modifiedEndpoints := endpoints.DeepCopy()
		modifiedEndpoints.Subsets = []v1.EndpointSubset{
			{
				Addresses: []v1.EndpointAddress{
					{IP: "1.2.3.4"},
				},
			},
		}
		// send a watch event (instead of setting replicas) so that the watch
		// func sees that it can forward the request now
		watcher.Modify(modifiedEndpoints)
		return nil
	})
	start := time.Now()
	r.NoError(grp.Wait())
	elapsed := time.Since(start)
	// assert here so that we can check all of these cases
	// rather than just failing at the first one
	a := assert.New(t)
	a.GreaterOrEqual(elapsed, sleepDur)
	a.Less(
		elapsed,
		sleepDur*2,
		"the handler took too long. this is usually because it timed out, not because it didn't find the watch event in time",
	)
	a.Equal(200, response.StatusCode)
}

func doRequest(
	cl *http.Client,
	urlStr,
	host string,
) (*http.Response, error) {
	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		return nil, err
	}
	req.Host = host
	res, err := cl.Do(req)
	if err != nil {
		return nil, err
	}
	return res, nil
}

type harness struct {
	lggr         logr.Logger
	proxyHdl     http.Handler
	proxySrv     *httptest.Server
	proxyURL     *url.URL
	originHdl    http.Handler
	originSrv    *httptest.Server
	originURL    *url.URL
	routingTable *routingtest.Table
	dialCtxFunc  kedanet.DialContextFunc
	endpCache    *k8s.FakeEndpointsCache
	waitFunc     forwardWaitFunc
}

func newHarness(
	t *testing.T,
	activeEndpointsTimeout time.Duration,
) (*harness, error) {
	t.Helper()
	lggr := logr.Discard()
	routingTable := routingtest.NewTable()
	dialContextFunc := kedanet.DialContextWithRetry(
		&net.Dialer{
			Timeout: 2 * time.Second,
		},
		wait.Backoff{
			Steps:    2,
			Duration: time.Second,
		},
	)

	svcCache := k8s.NewFakeServiceCache()
	endpCache := k8s.NewFakeEndpointsCache()
	waitFunc := newWorkloadReplicasForwardWaitFunc(
		logr.Discard(),
		endpCache,
	)

	originHdl := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte("hello!"))
		if err != nil {
			t.Fatalf("error writing message from origin: %s", err)
		}
	})
	testOriginSrv, originSrvURL, err := kedanet.StartTestServer(originHdl)
	if err != nil {
		return nil, err
	}

	proxyHdl := middleware.NewRouting(routingTable, nil, newForwardingHandler(
		lggr,
		dialContextFunc,
		waitFunc,
		forwardingConfig{
			waitTimeout:       activeEndpointsTimeout,
			respHeaderTimeout: time.Second,
		},
		&tls.Config{},
		&config.Tracing{}),
		svcCache,
		false,
	)

	proxySrv, proxySrvURL, err := kedanet.StartTestServer(proxyHdl)
	if err != nil {
		return nil, err
	}

	return &harness{
		lggr:         lggr,
		proxyHdl:     proxyHdl,
		proxySrv:     proxySrv,
		proxyURL:     proxySrvURL,
		originHdl:    originHdl,
		originSrv:    testOriginSrv,
		originURL:    originSrvURL,
		routingTable: routingTable,
		dialCtxFunc:  dialContextFunc,
		endpCache:    endpCache,
		waitFunc:     waitFunc,
	}, nil
}

func (h *harness) close() {
	h.proxySrv.Close()
	h.originSrv.Close()
}

func (h *harness) String() string {
	return fmt.Sprintf(
		"harness{proxy: %s, origin: %s}",
		h.proxyURL.String(),
		h.originURL.String(),
	)
}

func hostForTest(t *testing.T) string {
	t.Helper()
	return fmt.Sprintf("%s.integrationtest.interceptor.kedahttp.dev", t.Name())
}

// similar to net.SplitHostPort (https://pkg.go.dev/net#SplitHostPort)
// but returns the port as a string, not an int.
//
// useful because url.Host can contain the port, so ensure we only get the actual host
func splitHostPort(hostPortStr string) (string, int, error) {
	spl := strings.Split(hostPortStr, ":")
	if len(spl) != 2 {
		return "", 0, fmt.Errorf("invalid host:port: %s", hostPortStr)
	}
	host := spl[0]
	port, err := strconv.Atoi(spl[1])
	if err != nil {
		return "", 0, fmt.Errorf("port was invalid: %w", err)
	}
	return host, port, nil
}
