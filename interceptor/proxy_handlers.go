package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	apiappsv1 "k8s.io/api/apps/v1"
)

// deployFastGetter gets a Deployment quickly and without making a request
// directly to the Kubernetes API on each call to Get.
// It is backed by a single process that periodically updates the data in the background
type deployFastGetter interface {
	Get(name string) (*apiappsv1.Deployment, error)
}

// deployFastWatcher gets a stream of changes for the requested Deployment without
// making one request to the Kubernetes API on each call to Get.
// It is backed by a single process that opens the watch channel and broadcasts
// changes to all callers of Watch.
//
// On a call that returns a nil error, the caller must be sure to call the returned
// function to release any resources. On a call that returns a non-nil error, the other
// two return values will be nil
type deployFastWatcher interface {
	Watch(name string) (<-chan *apiappsv1.Deployment, func(), error)
}

type deployFastGetterWatcher interface {
	deployFastGetter
	deployFastWatcher
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
	deployIface deployFastGetterWatcher,
	deployName string,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Incoming request for %s", fwdSvcURL.String())

		deployment, err := deployIface.Get(deployName)
		if err != nil {
			log.Printf("Error getting deployment details, attempting to forward request anyway (%s)", err)
			forwardRequest(w, r, holdTimeout, fwdSvcURL)
			return
		}
		replicas := deployment.Spec.Replicas
		// if there is 1 or more replicas, forward the request immediately
		if replicas != nil && *replicas > 0 {
			forwardRequest(w, r, holdTimeout, fwdSvcURL)
			return
		}

		// otherwise, there is 0 replicas, so watch the deployment until it has
		// > 0 replicas or we time out
		ch, done, err := deployIface.Watch(deployName)
		if err != nil {
			// if the watch failed, try to forward the request once and bail out
			log.Printf("Error opening up watch stream for deployment, attempting to forward request anyway (%s)", err)
			forwardRequest(w, r, holdTimeout, fwdSvcURL)
			return
		}
		defer done()
		timer := time.NewTimer(holdTimeout)
		defer timer.Stop()
		for {
			select {
			case depl := <-ch:
				// if we got a deployment change and there is > 0 replicas this time, forward
				// the request and exit
				replicas := depl.Spec.Replicas
				if replicas != nil && *replicas > 0 {
					forwardRequest(w, r, holdTimeout, fwdSvcURL)
					return
				}
			case <-timer.C:
				// otherwise, if we hit the end of the timeout, try to forward the request
				// and exit
				log.Printf("Timeout expired waiting for deployment to reach > 0 replicas, attempting to forward request anyway (%s)", err)
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
