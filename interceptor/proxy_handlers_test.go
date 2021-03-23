package main

import (
	"testing"
	"time"
)

// make sure forwardRequest successfully forwards a request to a running server
func TestImmediatelySuccessfulProxy(t *testing.T) {
}

// make sure the forwarding handler doesn't forward if the origin didn't return
// within the hold timeout
// func TestForwardingHandlerNoForward(t *testing.T) {
// 	r := require.New(t)
// 	const respCode = 400
// 	const respBody = "you shouldn't see this!"
// 	var wg sync.WaitGroup
// 	wg.Add(1)
// 	originHdl := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 		wg.Wait()
// 		w.WriteHeader(respCode)
// 		w.Write([]byte(respBody))
// 	})

// 	testServer := httptest.NewServer(originHdl)
// 	defer testServer.Close()
// 	forwardURL, err := url.Parse(testServer.URL)
// 	r.NoError(err)

// 	const ns = "testNS"
// 	const deployName = "TestForwardingHandlerNoForwardDeploy"
// 	cache := k8s.NewMemoryDeploymentCache(map[string]*appsv1.Deployment{
// 		deployName: k8s.NewDeployment(
// 			ns,
// 			deployName,
// 			"myimage",
// 			[]int32{123},
// 			nil,
// 			map[string]string{},
// 		),
// 	})
// 	handler := newForwardingHandler(forwardURL, 100*time.Millisecond, cache, deployName)
// 	req, err := http.NewRequest("GET", "/testfwd", nil)
// 	r.NoError(err)
// 	rec := httptest.NewRecorder()
// 	handler.ServeHTTP(rec, req)

// 	// we should not get a 400 back from the proxy, even though that's
// 	// what the origin returns
// 	r.Equal(502, rec.Code, "response code from proxy")
// 	// nor should we get the handler's body back either. it should be
// 	// the proxy's error
// 	r.Contains(rec.Body.String(), "Error on backend")
// 	// tell the handler to be done so that we can close the server
// 	wg.Done()
// }

// // test to make sure that proxy will hold onto a request while an origin is working.
// // note: this is _not_ the same as a test to hold the request while a connection is being
// // established
// func TestForwardingHandlerWaitsForOrigin(t *testing.T) {
// 	r := require.New(t)
// 	const proxySleep = 1 * time.Second

// 	const respCode = 400
// 	const respBody = "you shouldn't see this!"
// 	// the handler will wait to do anything until this channel is closed
// 	handlerWaitCh := make(chan struct{})
// 	// the handler will close this channel after handlerWaitCh is closed
// 	// but before it sends the response
// 	reqRecvCh := make(chan struct{})
// 	originHdl := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 		<-handlerWaitCh
// 		log.Printf("running origin handler!")
// 		close(reqRecvCh)
// 		w.WriteHeader(respCode)
// 		w.Write([]byte(respBody))
// 		log.Printf("sent response from origin")
// 	})

// 	testServer := httptest.NewServer(originHdl)
// 	defer testServer.Close()
// 	forwardURL, err := url.Parse(testServer.URL)
// 	r.NoError(err)

// 	const ns = "testNS"
// 	const deployName = "TestForwardingHandlerWaitsDeployment"
// 	cache := k8s.NewMemoryDeploymentCache(map[string]*appsv1.Deployment{
// 		deployName: k8s.NewDeployment(
// 			ns,
// 			deployName,
// 			"myimage",
// 			[]int32{123},
// 			nil,
// 			map[string]string{},
// 		),
// 	})

// 	go func() {
// 		// sleep for 1/2 the time the proxy is gonna wait,
// 		// then close handlerWaitCh to signal the origin
// 		// to proceed
// 		time.Sleep(proxySleep / 2)
// 		close(handlerWaitCh)
// 	}()
// 	handler := newForwardingHandler(forwardURL, proxySleep, cache, deployName)

// 	req, err := http.NewRequest("GET", "/testfwd", nil)
// 	r.NoError(err)
// 	rec := httptest.NewRecorder()
// 	handler.ServeHTTP(rec, req)
// 	r.True(
// 		ensureSignalAfter(reqRecvCh, proxySleep),
// 		"origin didn't receive forwarded request within %s",
// 		proxySleep,
// 	)
// 	// we should not get a 400 back from the proxy, even though that's
// 	// what the origin returns
// 	r.Equal(
// 		respCode,
// 		rec.Code,
// 		"response code from proxy was wrong. body was %s",
// 		rec.Body.String(),
// 	)
// 	// nor should we get the handler's body back either. it should be
// 	// the proxy's error
// 	r.Equal(respBody, rec.Body.String(), "response body from proxy")
// }

// // test to make sure that proxy will wait for replicas, time out, and then
// // try to forward the request to the origin anyway
// func TestForwardingHandlerHoldsNoReplicas(t *testing.T) {
// 	r := require.New(t)
// 	const proxyTimeout = 1 * time.Second

// 	// this channel will be closed when the origin receives the request
// 	// (it should never be closed)
// 	originRecvCh := make(chan struct{})
// 	hdl := kedanet.NewTestHTTPHandlerWrapper(func(w http.ResponseWriter, r *http.Request) {
// 		close(originRecvCh)
// 	})
// 	originSrv := httptest.NewServer(hdl)
// 	defer originSrv.Close()
// 	forwardURL, err := url.Parse(originSrv.URL)
// 	r.NoError(err)

// 	// create a deployment and set the replicas to 0. We're not gonna
// 	// increase the number of replicas beyond 0. This will cause the proxy to hold
// 	// the request until its timeout, and then try to forward the request anyway
// 	const ns = "testNS"
// 	const deployName = "TestForwardingHandlerHoldsDeployment"
// 	deployment := k8s.NewDeployment(
// 		ns,
// 		deployName,
// 		"myimage",
// 		[]int32{123},
// 		nil,
// 		map[string]string{},
// 	)
// 	deployment.Spec.Replicas = k8s.Int32P(0)
// 	cache := k8s.NewMemoryDeploymentCache(map[string]*appsv1.Deployment{
// 		deployName: deployment,
// 	})
// 	handler := newForwardingHandler(forwardURL, proxyTimeout, cache, deployName)

// 	req, err := http.NewRequest("GET", "/testfwd", nil)
// 	r.NoError(err)
// 	rec := httptest.NewRecorder()
// 	handler.ServeHTTP(rec, req)

// 	// ensure that the origin still receives the request
// 	ensureSignalAfter(originRecvCh, proxyTimeout)

// 	forwardedRequests := hdl.IncomingRequests()
// 	r.Equal(
// 		0,
// 		len(forwardedRequests),
// 		"the origin received requests but it shouldn't have",
// 	)
// }

// // test to make sure that the proxy holds the request while there
// // are no replicas available on the origin deployment. Once there are
// // replicas available, it will then try to forward the request
// func TestForwardingHandlerHoldsUntilReplicas(t *testing.T) {
// 	r := require.New(t)
// 	const proxyTimeout = 1 * time.Second
// 	// this channel will be closed immediately after the request was
// 	// received at the origin, but before a response was sent
// 	originRequestCh := make(chan struct{})

// 	hdl := kedanet.NewTestHTTPHandlerWrapper(func(w http.ResponseWriter, r *http.Request) {
// 		close(originRequestCh)
// 		w.WriteHeader(200)
// 	})
// 	testSrv := httptest.NewServer(hdl)
// 	defer testSrv.Close()
// 	forwardURL, err := url.Parse(testSrv.URL)
// 	r.NoError(err)

// 	// create a deployment and set the replicas to 0. We'll not
// 	// be increasing the replicas beyond 0 so that the proxy holds the
// 	// request
// 	const ns = "testNS"
// 	const deployName = "TestForwardingHandlerHoldsDeployment"
// 	deployment := k8s.NewDeployment(
// 		ns,
// 		deployName,
// 		"myimage",
// 		[]int32{123},
// 		nil,
// 		map[string]string{},
// 	)
// 	deployment.Spec.Replicas = k8s.Int32P(0)
// 	cache := k8s.NewMemoryDeploymentCache(map[string]*appsv1.Deployment{
// 		deployName: deployment,
// 	})
// 	handler := newForwardingHandler(forwardURL, proxyTimeout, cache, deployName)

// 	req, err := http.NewRequest("GET", "/testfwd", nil)
// 	r.NoError(err)
// 	rec := httptest.NewRecorder()

// 	// this channel will be closed immediately after the replicas were increased
// 	replicasIncreasedCh := make(chan struct{})
// 	go func() {
// 		time.Sleep(proxyTimeout / 2)
// 		cache.RWM.RLock()
// 		defer cache.RWM.RUnlock()
// 		watcher := cache.Watchers[deployName]
// 		modifiedDeployment := deployment.DeepCopy()
// 		modifiedDeployment.Spec.Replicas = k8s.Int32P(1)
// 		watcher.Action(watch.Modified, modifiedDeployment)
// 		close(replicasIncreasedCh)
// 	}()
// 	handler.ServeHTTP(rec, req)

// 	r.True(
// 		ensureSignalAfter(replicasIncreasedCh, proxyTimeout/2),
// 		"replicas were not increased on origin deployment within %s",
// 		proxyTimeout/2,
// 	)
// 	r.True(
// 		ensureSignalAfter(originRequestCh, proxyTimeout),
// 		"origin did not receive forwarded request within %s",
// 		proxyTimeout,
// 	)
// }

// // test to make sure that the proxy tries and backs off trying to establish
// // a connection to an origin
// func TestForwardingHandlerRetriesConnection(t *testing.T) {

// }

// ensureSignalAfter ensures that signalCh receives (or is closed) before
// timeout
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

// ensuireNoSignalAfter ensures that signalCh does not receive (and is not closed)
// within timeout
func ensureNoSignalAfter(signalCh <-chan struct{}, timeout time.Duration) bool {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-signalCh:
		return false
	}
}
