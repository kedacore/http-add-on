package main

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
)

func forwardRequest(
	w http.ResponseWriter,
	r *http.Request,
	roundTripper http.RoundTripper,
	fwdSvcURL *url.URL,
) {
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
		// note: we can only use the '%w' directive inside of fmt.Errorf,
		// not Sprintf or anything similar. this means we have to create the
		// failure string in this slightly convoluted way.
		errMsg := fmt.Errorf("error on backend (%w)", err).Error()
		w.Write([]byte(errMsg))
	}

	proxy.ServeHTTP(w, r)
}
