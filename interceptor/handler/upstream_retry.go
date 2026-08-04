package handler

import (
	"errors"
	"net/http"

	"github.com/kedacore/http-add-on/interceptor/metrics"
	kedanet "github.com/kedacore/http-add-on/pkg/net"
	"github.com/kedacore/http-add-on/pkg/util"
)

// maxDialAttemptsPerEndpoint bounds how many times the dialer retries a single
// endpoint before the request may rotate to an alternate one. Kept small: a
// ready endpoint that refuses connections is almost always a pod that is
// terminating, so failing over fast beats waiting. Cold-start resilience for
// single-endpoint services is unaffected — with no alternate available the
// first attempt keeps the dialer's legacy context-bounded retry.
const maxDialAttemptsPerEndpoint = 3

// RetryConfig configures request retries against alternate upstream endpoints.
type RetryConfig struct {
	// Count is the maximum number of endpoint rotations per request.
	// A value <= 0 disables retries entirely.
	Count int
	// Instruments records the retry metric; nil disables it.
	Instruments *metrics.Instruments
}

// retryTransport retries a request against an alternate ready endpoint when
// the picked endpoint fails at TCP connect time — the steady-state case during
// scale-down, where a pod disappears between the endpoint snapshot and the
// dial. Only requests without a body are retried, and only on dial failure, so
// a retry can never replay a request the backend has already seen.
type retryTransport struct {
	base        http.RoundTripper
	retries     int
	instruments *metrics.Instruments
}

func newRetryTransport(base http.RoundTripper, cfg RetryConfig) *retryTransport {
	return &retryTransport{base: base, retries: cfg.Count, instruments: cfg.Instruments}
}

var _ http.RoundTripper = (*retryTransport)(nil)

func (rt *retryTransport) RoundTrip(req *http.Request) (resp *http.Response, err error) {
	picker := util.EndpointPickerFromContext(req.Context())
	if picker == nil || rt.retries <= 0 || !retryableRequest(req) {
		return rt.base.RoundTrip(req)
	}

	tried := map[string]struct{}{req.URL.Host: {}}
	rotated := false
	defer func() {
		if rotated && rt.instruments != nil {
			routeName, routeNamespace := "", ""
			if ir := util.InterceptorRouteFromContext(req.Context()); ir != nil {
				routeName, routeNamespace = ir.Name, ir.Namespace
			}
			rt.instruments.RecordRetry(routeName, routeNamespace, err == nil)
		}
	}()

	for rotations := 0; ; {
		attemptReq := req
		if rotations > 0 || picker(tried) != "" {
			// Bound this attempt's dials whenever a failover target may exist,
			// so a dead endpoint fails fast instead of soaking the whole
			// request budget redialing it. With no alternate in the snapshot,
			// the first attempt keeps the legacy unbounded dial retry, which
			// cold starts on single-replica services rely on.
			attemptReq = req.WithContext(kedanet.ContextWithMaxDialAttempts(req.Context(), maxDialAttemptsPerEndpoint))
		}

		resp, err := rt.base.RoundTrip(attemptReq)

		var exhausted *kedanet.DialAttemptsExhaustedError
		if !errors.As(err, &exhausted) {
			// Success, or a failure retrying cannot fix: the unbounded settle
			// attempt, a mid-request error, or a spent request budget.
			return resp, err
		}

		if rotations >= rt.retries {
			return nil, err
		}

		next := picker(tried)
		if next == "" {
			// The snapshot changed mid-flight; nothing left to rotate to.
			return nil, err
		}

		rotations++
		rotated = true
		tried[next] = struct{}{}

		util.LoggerFromContext(req.Context()).V(1).Info(
			"endpoint unreachable, retrying request against an alternate endpoint",
			"failedHost", req.URL.Host,
			"nextHost", next,
			"rotation", rotations,
		)

		req = requestWithHost(req, next)
	}
}

// retryableRequest reports whether the request can be re-dispatched without
// replaying a body: retries only trigger on connect failure, where no request
// byte reached the backend, so any body-less request is safe to resend.
func retryableRequest(req *http.Request) bool {
	return req.Body == nil || req.ContentLength == 0
}

// requestWithHost returns a copy of req with the URL host replaced, keeping
// everything else — including the context, so TLS SNI still resolves to the
// original service hostname.
func requestWithHost(req *http.Request, host string) *http.Request {
	r2 := req.Clone(req.Context())
	u := *req.URL
	u.Host = host
	r2.URL = &u
	return r2
}
