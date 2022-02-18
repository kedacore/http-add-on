package main

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
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
		// delete the incoming X-Forwarded-For header so the
		// proxy puts its own in. This is also important to
		// prevent IP spoofing
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

	proxy.ModifyResponse = func(resp *http.Response) error {
		locHdr := resp.Header.Get("Location")
		if locHdr == "" {
			// if there's no location header, no action
			// required
			return nil
		}
		// if there is a location header, and it has
		// no host in it, we need to rewrite that
		// a host in it, we need to rewrite that
		// host to the originally requested one
		locURL, err := url.Parse(locHdr)
		if err != nil {
			return err
		}

		// if the location header has no host,
		// add the incoming host to it
		if locURL.Host == "" {
			locURL.Host = r.Host
			locURL.Scheme = ""
			loc := strings.TrimPrefix(locURL.String(), "//")
			resp.Header.Set("Location", loc)
		}

		return nil
	}

	proxy.ServeHTTP(w, r)
}
