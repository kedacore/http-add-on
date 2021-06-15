package main

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	kedanet "github.com/kedacore/http-add-on/pkg/net"
)

func forwardRequest(
	w http.ResponseWriter,
	r *http.Request,
	dialCtxFunc kedanet.DialContextFunc,
	respHeaderTimeout time.Duration,
	fwdSvcURL *url.URL,
) {
	roundTripper := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           dialCtxFunc,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ResponseHeaderTimeout: respHeaderTimeout,
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
		errMsg := fmt.Errorf("error on backend (%w)", err).Error()
		w.Write([]byte(errMsg))
	}

	proxy.ServeHTTP(w, r)
}
