package main

import (
	"context"
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
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	httpv1alpha1 "github.com/kedacore/http-add-on/operator/apis/http/v1alpha1"
	"github.com/kedacore/http-add-on/pkg/k8s"
	kedanet "github.com/kedacore/http-add-on/pkg/net"
	"github.com/kedacore/http-add-on/pkg/routing"
)

// happy path - deployment is scaled to 1 and host in routing table
func TestIntegrationHappyPath(t *testing.T) {
	const (
		deploymentReplicasTimeout = 200 * time.Millisecond
		responseHeaderTimeout     = 1 * time.Second
		deplName                  = "testdeployment"
	)
	r := require.New(t)
	h, err := newHarness(
		t,
		deploymentReplicasTimeout,
	)
	r.NoError(err)
	defer h.close()
	t.Logf("Harness: %s", h.String())

	originPort, err := strconv.Atoi(h.originURL.Port())
	r.NoError(err)

	target := targetFromURL(
		h.originURL,
		originPort,
		deplName,
	)
	h.routingTable.(*testRoutingTable).memory[hostForTest(t)] = target

	h.deplCache.Set(target.GetNamespace(), deplName, appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: deplName},
		Spec: appsv1.DeploymentSpec{
			// note that the forwarding wait function doesn't care about
			// the replicas field, it only cares about ReadyReplicas in the status.
			// regardless, we're setting this because in a running cluster,
			// it's likely that most of the time, this is equal to ReadyReplicas
			Replicas: i32Ptr(3),
		},
		Status: appsv1.DeploymentStatus{
			ReadyReplicas: 3,
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
	res.Body.Close()
}

// deployment scaled to 1 but host not in routing table
//
// NOTE: the interceptor needs to check in the routing table
// _before_ checking the deployment cache, so we don't technically
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
		deployTimeout = 100 * time.Millisecond
	)
	host := hostForTest(t)
	deployName := "testdeployment"
	r := require.New(t)
	h, err := newHarness(t, deployTimeout)
	r.NoError(err)

	originPort, err := strconv.Atoi(h.originURL.Port())
	r.NoError(err)

	target := targetFromURL(
		h.originURL,
		originPort,
		deployName,
	)
	h.routingTable.(*testRoutingTable).memory[hostForTest(t)] = target

	// 0 replicas
	h.deplCache.Set(target.GetNamespace(), deployName, appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: deployName},
		Spec: appsv1.DeploymentSpec{
			Replicas: i32Ptr(0),
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
	// we should have slept more than the deployment replicas wait timeout
	r.GreaterOrEqual(elapsed, deployTimeout)
	r.Less(elapsed, deployTimeout+50*time.Millisecond)
}

// the request comes in while there are no replicas, and one is added
// while it's pending
func TestIntegrationWaitReplicas(t *testing.T) {
	const (
		deployTimeout   = 2 * time.Second
		responseTimeout = 1 * time.Second
		deployName      = "testdeployment"
	)
	ctx := context.Background()
	r := require.New(t)
	h, err := newHarness(t, deployTimeout)
	r.NoError(err)

	// add host to routing table
	originPort, err := strconv.Atoi(h.originURL.Port())
	r.NoError(err)

	target := targetFromURL(
		h.originURL,
		originPort,
		deployName,
	)
	h.routingTable.(*testRoutingTable).memory[hostForTest(t)] = target

	// set up a deployment with zero replicas and create
	// a watcher we can use later to fake-send a deployment
	// event
	initialDeployment := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: deployName},
		Spec: appsv1.DeploymentSpec{
			Replicas: i32Ptr(0),
		},
	}
	h.deplCache.Set(target.GetNamespace(), deployName, initialDeployment)
	watcher := h.deplCache.SetWatcher(target.GetNamespace(), deployName)

	// make the request in one goroutine, and in the other, wait a bit
	// and then add replicas to the deployment cache

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
	const sleepDur = deployTimeout / 4
	grp.Go(func() error {
		t.Logf("Sleeping for %s", sleepDur)
		time.Sleep(sleepDur)
		t.Logf("Woke up, setting replicas to 10")
		modifiedDeployment := initialDeployment.DeepCopy()
		// note that the wait function only cares about Status.ReadyReplicas
		// but we're setting Spec.Replicas to 10 as well because the common
		// case in the cluster is that they would be equal
		modifiedDeployment.Spec.Replicas = i32Ptr(10)
		modifiedDeployment.Status.ReadyReplicas = 10
		// send a watch event (instead of setting replicas) so that the watch
		// func sees that it can forward the request now
		watcher.Modify(modifiedDeployment)
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
	routingTable routing.Table
	dialCtxFunc  kedanet.DialContextFunc
	deplCache    *k8s.FakeDeploymentCache
	waitFunc     forwardWaitFunc
}

func newHarness(
	t *testing.T,
	deployReplicasTimeout time.Duration,
) (*harness, error) {
	t.Helper()
	lggr := logr.Discard()
	routingTable := newTestRoutingTable()
	dialContextFunc := kedanet.DialContextWithRetry(
		&net.Dialer{
			Timeout: 2 * time.Second,
		},
		wait.Backoff{
			Steps:    2,
			Duration: time.Second,
		},
	)

	deplCache := k8s.NewFakeDeploymentCache()
	waitFunc := newDeployReplicasForwardWaitFunc(
		logr.Discard(),
		deplCache,
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

	proxyHdl := newForwardingHandler(
		lggr,
		routingTable,
		dialContextFunc,
		waitFunc,
		forwardingConfig{
			waitTimeout:       deployReplicasTimeout,
			respHeaderTimeout: time.Second,
		},
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
		deplCache:    deplCache,
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

func i32Ptr(i int32) *int32 {
	return &i
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
		return "", 0, errors.Wrap(err, "port was invalid")
	}
	return host, port, nil
}

type testRoutingTable struct {
	memory map[string]*httpv1alpha1.HTTPScaledObject
}

func newTestRoutingTable() *testRoutingTable {
	return &testRoutingTable{
		memory: make(map[string]*httpv1alpha1.HTTPScaledObject),
	}
}

var _ routing.Table = (*testRoutingTable)(nil)

func (t testRoutingTable) Start(_ context.Context) error {
	return nil
}

func (t testRoutingTable) Route(req *http.Request) *httpv1alpha1.HTTPScaledObject {
	return t.memory[req.Host]
}

func (t testRoutingTable) HasSynced() bool {
	return true
}
