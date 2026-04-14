package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
	discov1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/kedacore/http-add-on/interceptor/config"
	"github.com/kedacore/http-add-on/interceptor/metrics"
	httpv1beta1 "github.com/kedacore/http-add-on/operator/apis/http/v1beta1"
	kedacache "github.com/kedacore/http-add-on/pkg/cache"
	"github.com/kedacore/http-add-on/pkg/k8s"
	kedanet "github.com/kedacore/http-add-on/pkg/net"
	"github.com/kedacore/http-add-on/pkg/queue"
	routingtest "github.com/kedacore/http-add-on/pkg/routing/test"
)

// happy path - deployment is scaled to 1 and host in routing table
func TestIntegrationHappyPath(t *testing.T) {
	const (
		activeEndpointsTimeout = 200 * time.Millisecond
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
		originPort,
		serviceName,
	)
	h.routingTable.Memory[hostForTest(t)] = target

	h.readyCache.Update(target.Namespace+"/"+serviceName, []*discov1.EndpointSlice{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      serviceName + "-slice",
				Namespace: target.Namespace,
				Labels:    map[string]string{discov1.LabelServiceName: serviceName},
			},
			Endpoints: []discov1.Endpoint{{Addresses: []string{"1.2.3.4"}}},
		},
	})

	// happy path
	res, err := doRequest(
		http.DefaultClient,
		h.proxyURL.String(),
		hostForTest(t),
	)
	r.NoError(err)
	r.Equal(200, res.StatusCode)
	_ = res.Body.Close()
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
	_ = res.Body.Close()
	r.NoError(err)
	r.Equal(404, res.StatusCode)
	_ = res.Body.Close()
}

// host in the routing table but deployment has no replicas
func TestIntegrationNoReplicas(t *testing.T) {
	const (
		activeEndpointsTimeout = 100 * time.Millisecond
	)
	host := hostForTest(t)
	serviceName := "testservice"
	r := require.New(t)
	h, err := newHarness(t, activeEndpointsTimeout)
	r.NoError(err)

	originPort, err := strconv.Atoi(h.originURL.Port())
	r.NoError(err)

	target := targetFromURL(
		originPort,
		serviceName,
	)
	h.routingTable.Memory[hostForTest(t)] = target

	// 0 replicas — don't update the ready cache, so WaitForReady will time out

	start := time.Now()
	res, err := doRequest(
		http.DefaultClient,
		h.proxyURL.String(),
		host,
	)
	r.NoError(err)
	r.Equal(504, res.StatusCode)
	_ = res.Body.Close()
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
		originPort,
		serviceName,
	)
	h.routingTable.Memory[hostForTest(t)] = target

	// Start with zero replicas — don't update the ready cache yet.
	// Make the request in one goroutine, and in the other, wait a bit
	// and then add replicas via the ready cache.

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
		_ = resp.Body.Close()
		return nil
	})
	const sleepDur = activeEndpointsTimeout / 4
	grp.Go(func() error {
		t.Logf("Sleeping for %s", sleepDur)
		time.Sleep(sleepDur)
		t.Logf("Woke up, setting replicas to 10")

		h.readyCache.Update(target.Namespace+"/"+serviceName, []*discov1.EndpointSlice{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      serviceName + "-slice",
					Namespace: target.Namespace,
					Labels:    map[string]string{discov1.LabelServiceName: serviceName},
				},
				Endpoints: []discov1.Endpoint{{Addresses: []string{"1.2.3.4"}}},
			},
		})
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
	readyCache   *k8s.ReadyEndpointsCache
}

func newHarness(
	t *testing.T,
	activeEndpointsTimeout time.Duration,
) (*harness, error) {
	t.Helper()
	lggr := logr.Discard()
	routingTable := routingtest.NewTable()

	fakeClient := fake.NewClientBuilder().WithScheme(kedacache.NewScheme()).Build()
	readyCache := k8s.NewReadyEndpointsCache(logr.Discard())

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

	proxyHdl := BuildProxyHandler(&ProxyHandlerConfig{
		Logger:       lggr,
		Queue:        queue.NewMemory(),
		ReadyCache:   readyCache,
		RoutingTable: routingTable,
		Reader:       fakeClient,
		Timeouts: config.Timeouts{
			Connect:        100 * time.Millisecond,
			Readiness:      activeEndpointsTimeout,
			Request:        60 * time.Second,
			ResponseHeader: time.Second,
		},
		Serving:             config.Serving{},
		Instruments:         metrics.NewNoopInstruments(),
		dialAddressOverride: originSrvURL.Host,
	})

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
		readyCache:   readyCache,
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

func targetFromURL(
	port int,
	service string,
) *httpv1beta1.InterceptorRoute {
	return &httpv1beta1.InterceptorRoute{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test-namespace",
		},
		Spec: httpv1beta1.InterceptorRouteSpec{
			Target: httpv1beta1.TargetRef{
				Service: service,
				Port:    int32(port),
			},
		},
	}
}
