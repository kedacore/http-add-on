package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"
)

// newForwardingHandler takes in the service URL for the app backend
// and forwards incoming requests to it. Note that it isn't multitenant.
// It's intended to be deployed and scaled alongside the application itself
func newForwardingHandler(fwdSvcURL *url.URL, holdTimeout time.Duration) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Incoming request for %s", fwdSvcURL.String())

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
	})
}
