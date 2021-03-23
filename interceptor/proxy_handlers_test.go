package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/kedacore/http-add-on/pkg/k8s"
	kedanet "github.com/kedacore/http-add-on/pkg/net"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/watch"
)

// make sure the forwarding handler forwards normally with a
// reasonable forward timeout
func TestForwardingHandler(t *testing.T) {
	r := require.New(t)
	var wg sync.WaitGroup
	wg.Add(1)
	forwardedRequests := []*http.Request{}
	const respCode = 302
	const respBody = "TestForwardingHandler"
	originHdl := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer wg.Done()
		forwardedRequests = append(forwardedRequests, r)
		w.WriteHeader(respCode)
		w.Write([]byte(respBody))
	})
	testServer := httptest.NewServer(originHdl)
	defer testServer.Close()
	forwardURL, err := url.Parse(testServer.URL)
	r.NoError(err)

	const ns = "testNS"
	const deployName = "TestForwardingHandlerDeploy"
	cache := k8s.NewMemoryDeploymentCache(map[string]*appsv1.Deployment{
		deployName: k8s.NewDeployment(
			ns,
			deployName,
			"myimage",
			[]int32{123},
			nil,
			map[string]string{},
		),
	})
	handler := newForwardingHandler(forwardURL, 1*time.Second, cache, deployName)
	const path = "/testfwd"
	req, err := http.NewRequest("GET", path, nil)
	r.NoError(err)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	r.Equal(1, len(forwardedRequests), "number of requests forwarded")
	forwardedRequest := forwardedRequests[0]
	r.Equal(path, forwardedRequest.URL.Path)
	r.Equal(302, rec.Code)
	r.Equal(respBody, rec.Body.String())
}

// make sure the forwarding handler doesn't forward if the origin didn't return
// within the hold timeout
func TestForwardingHandlerNoForward(t *testing.T) {
	r := require.New(t)
	const respCode = 400
	const respBody = "you shouldn't see this!"
	var wg sync.WaitGroup
	wg.Add(1)
	originHdl := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wg.Wait()
		w.WriteHeader(respCode)
		w.Write([]byte(respBody))
	})

	testServer := httptest.NewServer(originHdl)
	defer testServer.Close()
	forwardURL, err := url.Parse(testServer.URL)
	r.NoError(err)

	const ns = "testNS"
	const deployName = "TestForwardingHandlerNoForwardDeploy"
	cache := k8s.NewMemoryDeploymentCache(map[string]*appsv1.Deployment{
		deployName: k8s.NewDeployment(
			ns,
			deployName,
			"myimage",
			[]int32{123},
			nil,
			map[string]string{},
		),
	})
	handler := newForwardingHandler(forwardURL, 100*time.Millisecond, cache, deployName)
	req, err := http.NewRequest("GET", "/testfwd", nil)
	r.NoError(err)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// we should not get a 400 back from the proxy, even though that's
	// what the origin returns
	r.Equal(502, rec.Code, "response code from proxy")
	// nor should we get the handler's body back either. it should be
	// the proxy's error
	r.Contains(rec.Body.String(), "Error on backend")
	// tell the handler to be done so that we can close the server
	wg.Done()
}

// test to make sure that proxy will hold onto a request while an origin is working.
// note: this is _not_ the same as a test to hold the request while a connection is being
// established
func TestForwardingHandlerWaitsForOrigin(t *testing.T) {
	r := require.New(t)
	// have the origin sleep for less time than the interceptor
	// will wait
	const originSleep = 200 * time.Millisecond
	const proxySleep = 1 * time.Second

	const respCode = 400
	const respBody = "you shouldn't see this!"
	originHdl := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(originSleep)
		w.WriteHeader(respCode)
		w.Write([]byte(respBody))
	})

	testServer := httptest.NewServer(originHdl)
	defer testServer.Close()
	forwardURL, err := url.Parse(testServer.URL)
	r.NoError(err)

	const ns = "testNS"
	const deployName = "TestForwardingHandlerWaitsDeployment"
	cache := k8s.NewMemoryDeploymentCache(map[string]*appsv1.Deployment{
		deployName: k8s.NewDeployment(
			ns,
			deployName,
			"myimage",
			[]int32{123},
			nil,
			map[string]string{},
		),
	})
	handler := newForwardingHandler(forwardURL, proxySleep, cache, deployName)

	req, err := http.NewRequest("GET", "/testfwd", nil)
	r.NoError(err)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// we should not get a 400 back from the proxy, even though that's
	// what the origin returns
	r.Equal(respCode, rec.Code, "response code from proxy")
	// nor should we get the handler's body back either. it should be
	// the proxy's error
	r.Equal(respBody, rec.Body.String(), "response body from proxy")
}

// test to make sure that proxy will wait and never try to establish a connection
// to the origin while there are no replicas, and eventually time out
func TestForwardingHandlerHoldsNoReplicas(t *testing.T) {
	r := require.New(t)
	const proxyTimeout = 1 * time.Second

	forwardURL, err := url.Parse("https://asfgsdfgkjdfg.dev")
	r.NoError(err)

	// create a deployment and set the replicas to 0. We'll not
	// be increasing the replicas beyond 0 so that the proxy holds the
	// request
	const ns = "testNS"
	const deployName = "TestForwardingHandlerHoldsDeployment"
	deployment := k8s.NewDeployment(
		ns,
		deployName,
		"myimage",
		[]int32{123},
		nil,
		map[string]string{},
	)
	deployment.Spec.Replicas = k8s.Int32P(0)
	cache := k8s.NewMemoryDeploymentCache(map[string]*appsv1.Deployment{
		deployName: deployment,
	})
	handler := newForwardingHandler(forwardURL, proxyTimeout, cache, deployName)

	req, err := http.NewRequest("GET", "/testfwd", nil)
	r.NoError(err)
	rec := httptest.NewRecorder()

	reqStart := time.Now()
	handler.ServeHTTP(rec, req)
	reqDur := time.Since(reqStart)
	r.True(
		reqDur >= proxyTimeout,
		"request duration (%s) wasn't >= than the proxy's wait time (%s)",
		reqDur,
		proxyTimeout,
	)
}

// test to make sure that the proxy holds the request while there
// are no replicas available on the origin deployment. Once there are
// replicas available, it will then try to forward the request
func TestForwardingHandlerHoldsUntilReplicas(t *testing.T) {
	r := require.New(t)
	const proxyTimeout = 1 * time.Second
	// this channel will be closed immediately after the request was
	// received at the origin, but before a response was sent
	originRequestCh := make(chan struct{})

	hdl := kedanet.NewTestHTTPHandlerWrapper(func(w http.ResponseWriter, r *http.Request) {
		close(originRequestCh)
		w.WriteHeader(200)
	})
	testSrv := httptest.NewServer(hdl)
	defer testSrv.Close()
	forwardURL, err := url.Parse(testSrv.URL)
	r.NoError(err)

	// create a deployment and set the replicas to 0. We'll not
	// be increasing the replicas beyond 0 so that the proxy holds the
	// request
	const ns = "testNS"
	const deployName = "TestForwardingHandlerHoldsDeployment"
	deployment := k8s.NewDeployment(
		ns,
		deployName,
		"myimage",
		[]int32{123},
		nil,
		map[string]string{},
	)
	deployment.Spec.Replicas = k8s.Int32P(0)
	cache := k8s.NewMemoryDeploymentCache(map[string]*appsv1.Deployment{
		deployName: deployment,
	})
	handler := newForwardingHandler(forwardURL, proxyTimeout, cache, deployName)

	req, err := http.NewRequest("GET", "/testfwd", nil)
	r.NoError(err)
	rec := httptest.NewRecorder()

	// this channel will be closed immediately after the replicas were increased
	replicasIncreasedCh := make(chan struct{})
	go func() {
		time.Sleep(proxyTimeout / 2)
		cache.RWM.RLock()
		defer cache.RWM.RUnlock()
		watcher := cache.Watchers[deployName]
		modifiedDeployment := deployment.DeepCopy()
		modifiedDeployment.Spec.Replicas = k8s.Int32P(1)
		watcher.Action(watch.Modified, modifiedDeployment)
		close(replicasIncreasedCh)
	}()
	handler.ServeHTTP(rec, req)

	r.True(
		ensureSignalAfter(replicasIncreasedCh, proxyTimeout/2),
		"replicas were not increased on origin deployment within %s",
		proxyTimeout/2,
	)
	r.True(
		ensureSignalAfter(originRequestCh, proxyTimeout),
		"origin did not receive forwarded request within %s",
		proxyTimeout,
	)
}

// test to make sure that the proxy tries and backs off trying to establish
// a connection to an origin
func TestForwardingHandlerRetriesConnection(t *testing.T) {

}

func ensureSignalAfter(signalCh <-chan struct{}, timeout time.Duration) bool {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-timer.C:
		return false
	case <-signalCh:
		return true
	}
}
