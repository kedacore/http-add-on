package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/kedacore/http-add-on/pkg/k8s"
	kedanet "github.com/kedacore/http-add-on/pkg/net"
	"github.com/kedacore/http-add-on/pkg/routing"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

// happy path - deployment is scaled to 1 and host in routing table
func TestIntegrationHappyPath(t *testing.T) {
	const (
		deploymentReplicasTimeout = 200 * time.Millisecond
		responseHeaderTimeout     = 1 * time.Second
		deplName                  = "testdeployment"
	)
	r := require.New(t)
	h, err := newHarness(deploymentReplicasTimeout, responseHeaderTimeout)
	r.NoError(err)
	defer h.close()
	t.Logf("Harness: %s", h.String())

	h.deplCache.Set(deplName, appsv1.Deployment{
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

	originPort, err := strconv.Atoi(h.originURL.Port())
	r.NoError(err)
	h.routingTable.AddTarget(hostForTest(t), targetFromURL(
		h.originURL,
		originPort,
		deplName,
		123,
	))

	// happy path
	res, err := doRequest(
		http.DefaultClient,
		"GET",
		h.proxyURL.String(),
		hostForTest(t),
		nil,
	)
	r.NoError(err)
	r.Equal(200, res.StatusCode)
}

// deployment scaled to 1 but host not in routing table
//
// NOTE: the interceptor needs to check in the routing table
// _before_ checking the deployment cache, so we don't technically
// need to set the replicas to 1, but we're doing so anyway to
// isolate the routing table behavior
func TestIntegrationNoRoutingTableEntry(t *testing.T) {
	host := fmt.Sprintf("%s.integrationtest.interceptor.kedahttp.dev", t.Name())
	r := require.New(t)
	h, err := newHarness(time.Second, time.Second)
	r.NoError(err)
	defer h.close()
	h.deplCache.Set(host, appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: host},
		Spec: appsv1.DeploymentSpec{
			Replicas: i32Ptr(1),
		},
	})

	// not in the routing table
	res, err := doRequest(
		http.DefaultClient,
		"GET",
		h.proxyURL.String(),
		"not-in-the-table",
		nil,
	)
	r.NoError(err)
	r.Equal(404, res.StatusCode)
}

// host in the routing table but deployment has no replicas
func TestIntegrationNoReplicas(t *testing.T) {
	const (
		deployTimeout = 100 * time.Millisecond
	)
	host := hostForTest(t)
	deployName := "testdeployment"
	r := require.New(t)
	h, err := newHarness(deployTimeout, time.Second)
	r.NoError(err)

	originPort, err := strconv.Atoi(h.originURL.Port())
	r.NoError(err)
	h.routingTable.AddTarget(hostForTest(t), targetFromURL(
		h.originURL,
		originPort,
		deployName,
		123,
	))

	// 0 replicas
	h.deplCache.Set(deployName, appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: deployName},
		Spec: appsv1.DeploymentSpec{
			Replicas: i32Ptr(0),
		},
	})

	start := time.Now()
	res, err := doRequest(
		http.DefaultClient,
		"GET",
		h.proxyURL.String(),
		host,
		nil,
	)

	r.NoError(err)
	r.Equal(502, res.StatusCode)
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
	h, err := newHarness(deployTimeout, responseTimeout)
	r.NoError(err)

	// add host to routing table
	originPort, err := strconv.Atoi(h.originURL.Port())
	r.NoError(err)
	h.routingTable.AddTarget(
		hostForTest(t),
		targetFromURL(
			h.originURL,
			originPort,
			deployName,
			123,
		),
	)

	// set up a deployment with zero replicas and create
	// a watcher we can use later to fake-send a deployment
	// event
	initialDeployment := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: deployName},
		Spec: appsv1.DeploymentSpec{
			Replicas: i32Ptr(0),
		},
	}
	h.deplCache.Set(deployName, initialDeployment)
	watcher := h.deplCache.SetWatcher(deployName)

	// make the request in one goroutine, and in the other, wait a bit
	// and then add replicas to the deployment cache

	var response *http.Response
	grp, _ := errgroup.WithContext(ctx)
	grp.Go(func() error {
		resp, err := doRequest(
			http.DefaultClient,
			"GET",
			h.proxyURL.String(),
			hostForTest(t),
			nil,
		)
		if err != nil {
			return err
		}
		response = resp

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
	method,
	urlStr,
	host string,
	body io.ReadCloser,
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
	routingTable *routing.Table
	dialCtxFunc  kedanet.DialContextFunc
	deplCache    *k8s.FakeDeploymentCache
	waitFunc     forwardWaitFunc
}

func newHarness(
	deployReplicasTimeout,
	responseHeaderTimeout time.Duration,
) (*harness, error) {
	lggr := logr.Discard()
	routingTable := routing.NewTable()
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
	waitFunc := newDeployReplicasForwardWaitFunc(deplCache)

	proxyHdl := newForwardingHandler(
		lggr,
		routingTable,
		dialContextFunc,
		waitFunc,
		forwardingConfig{
			waitTimeout:       deployReplicasTimeout,
			respHeaderTimeout: responseHeaderTimeout,
		},
	)

	originHdl := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("hello!"))
	})
	testOriginSrv, originSrvURL, err := kedanet.StartTestServer(originHdl)
	if err != nil {
		return nil, err
	}

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
