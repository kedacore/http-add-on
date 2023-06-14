package handler

import (
	"errors"
	"net/http"
	"net/http/httputil"

	"github.com/kedacore/http-add-on/pkg/util"
)

var (
	errNilStream = errors.New("context stream is nil")
)

type Upstream struct {
	roundTripper http.RoundTripper
}

func NewUpstream(roundTripper http.RoundTripper) *Upstream {
	return &Upstream{
		roundTripper: roundTripper,
	}
}

var _ http.Handler = (*Upstream)(nil)

func (uh *Upstream) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	r = util.RequestWithLoggerWithName(r, "UpstreamHandler")
	ctx := r.Context()

	stream := util.StreamFromContext(ctx)
	if stream == nil {
		sh := NewStatic(http.StatusInternalServerError, errNilStream)
		sh.ServeHTTP(w, r)

		return
	}

	proxy := httputil.NewSingleHostReverseProxy(stream)
	proxy.Transport = uh.roundTripper
	proxy.Director = func(req *http.Request) {
		req.URL = stream
		req.Host = stream.Host
		req.URL.Path = r.URL.Path
		req.URL.RawQuery = r.URL.RawQuery
		// delete the incoming X-Forwarded-For header so the proxy
		// puts its own in. This is also important to prevent IP spoofing
		req.Header.Del("X-Forwarded-For ")
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		sh := NewStatic(http.StatusBadGateway, err)
		sh.ServeHTTP(w, r)
	}

	proxy.ServeHTTP(w, r)
}
