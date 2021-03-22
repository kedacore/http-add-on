package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/kedacore/http-add-on/pkg/k8s"
	appsv1 "k8s.io/api/apps/v1"
)

func moreThanPtr(i *int32, target int32) bool {
	return i != nil && *i > target
}

// newForwardingHandler takes in the service URL for the app backend
// and forwards incoming requests to it. Note that it isn't multitenant.
// It's intended to be deployed and scaled alongside the application itself.
//
// fwdSvcURL must have a valid scheme in it. The best way to do this is
// create a URL with url.Parse("https://...")
func newForwardingHandler(
	fwdSvcURL *url.URL,
	holdTimeout time.Duration,
	deployCache k8s.DeploymentCache,
	deployName string,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Incoming request for %s", fwdSvcURL.String())

		deployment, err := deployCache.Get(deployName)
		if err != nil {
			// if we didn't get the initial deployment state, just log it.
			// below we're going to watch the deployment change stream anyway
			log.Printf("Error getting state for deployment %s (%s)", deployName, err)
		} else {
			// if there is 1 or more replica, forward the request immediately
			if moreThanPtr(deployment.Spec.Replicas, 0) {
				forwardRequest(w, r, holdTimeout, fwdSvcURL)
				return
			}
		}

		watcher := deployCache.Watch(deployName)
		if err != nil {
			log.Printf("Error getting the stream of deployment changes")
		}
		defer watcher.Stop()
		eventCh := watcher.ResultChan()
		timer := time.NewTimer(holdTimeout)
		defer timer.Stop()
		for {
			select {
			case event := <-eventCh:
				deployment := event.Object.(*appsv1.Deployment)
				if err != nil {
					log.Printf(
						"Error getting deployment %s after change was triggered (%s)",
						deployName,
						err,
					)
				}
				if moreThanPtr(deployment.Spec.Replicas, 0) {
					forwardRequest(w, r, holdTimeout, fwdSvcURL)
					return
				}
			case <-timer.C:
				// otherwise, if we hit the end of the timeout, try to forward the request
				// and exit
				log.Printf("Timeout expired waiting for deployment to reach > 0 replicas, attempting to forward request anyway")
				forwardRequest(w, r, holdTimeout, fwdSvcURL)
				return
			}
		}
	})
}

func forwardRequest(
	w http.ResponseWriter,
	r *http.Request,
	holdTimeout time.Duration,
	fwdSvcURL *url.URL,
) {
	// this is adapted from https://pkg.go.dev/net/http#RoundTripper
	roundTripper := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   holdTimeout,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ResponseHeaderTimeout: holdTimeout,
	}
	proxy := httputil.NewSingleHostReverseProxy(fwdSvcURL)
	proxy.Transport = roundTripper
	proxy.Director = func(req *http.Request) {
		req.URL = fwdSvcURL
		req.Host = fwdSvcURL.Host
		req.URL.Path = r.URL.Path
		req.URL.RawQuery = r.URL.RawQuery
		// delete the incoming X-Forwarded-For header so the proxy
		// puts its own in. This is also important to prevent IP spoofing
		req.Header.Del("X-Forwarded-For ")
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		w.WriteHeader(502)
		errMsg := fmt.Sprintf("Error on backend (%s)", err)
		w.Write([]byte(errMsg))
	}

	proxy.ServeHTTP(w, r)
}
