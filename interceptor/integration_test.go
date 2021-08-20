package main

import (
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
	"github.com/stretchr/testify/require"
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

	h.deplCache.Set(deplName, &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: deplName},
		Spec: appsv1.DeploymentSpec{
			Replicas: i32Ptr(3),
		},
	})

	originPortInt, err := strconv.Atoi(h.originURL.Port())
	r.NoError(err)
	h.routingTable.AddTarget(hostForTest(t), routing.Target{
		// url.Host can contain the port, so ensure we only get the actual host
		Service:    strings.Split(h.originURL.Host, ":")[0],
		Port:       originPortInt,
		Deployment: deplName,
	})

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
	h.deplCache.Set(host, &appsv1.Deployment{
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
	host := hostForTest(t)
	deployName := "testdeployment"
	r := require.New(t)
	h, err := newHarness(100*time.Millisecond, time.Second)
	r.NoError(err)

	h.routingTable.AddTarget(hostForTest(t), routing.Target{
		Service:    hostForTest(t),
		Port:       80,
		Deployment: deployName,
	})

	// 0 replicas
	h.deplCache.Set(deployName, &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: deployName},
		Spec: appsv1.DeploymentSpec{
			Replicas: i32Ptr(0),
		},
	})
	h.deplCache.SetReplicas(hostForTest(t), 0)

	res, err := doRequest(
		http.DefaultClient,
		"GET",
		h.proxyURL.String(),
		host,
		nil,
	)
	r.NoError(err)
	r.Equal(502, res.StatusCode)
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
		deployReplicasTimeout,
		responseHeaderTimeout,
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
