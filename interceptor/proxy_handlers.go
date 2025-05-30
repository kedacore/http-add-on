package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-logr/logr"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/kedacore/http-add-on/interceptor/config"
	"github.com/kedacore/http-add-on/interceptor/handler"
	kedahttp "github.com/kedacore/http-add-on/pkg/http"
	"github.com/kedacore/http-add-on/pkg/k8s"
	kedanet "github.com/kedacore/http-add-on/pkg/net"
	"github.com/kedacore/http-add-on/pkg/util"
)

type forwardingConfig struct {
	waitTimeout           time.Duration
	respHeaderTimeout     time.Duration
	forceAttemptHTTP2     bool
	maxIdleConns          int
	maxIdleConnsPerHost   int
	idleConnTimeout       time.Duration
	tlsHandshakeTimeout   time.Duration
	expectContinueTimeout time.Duration
	enableColdStartHeader bool
}

func newForwardingConfigFromTimeouts(t *config.Timeouts, s *config.Serving) forwardingConfig {
	return forwardingConfig{
		waitTimeout:           t.WorkloadReplicas,
		respHeaderTimeout:     t.ResponseHeader,
		forceAttemptHTTP2:     t.ForceHTTP2,
		maxIdleConns:          t.MaxIdleConns,
		maxIdleConnsPerHost:   t.MaxIdleConnsPerHost,
		idleConnTimeout:       t.IdleConnTimeout,
		tlsHandshakeTimeout:   t.TLSHandshakeTimeout,
		expectContinueTimeout: t.ExpectContinueTimeout,
		enableColdStartHeader: s.EnableColdStartHeader,
	}
}

// newForwardingHandler takes in the service URL for the app backend
// and forwards incoming requests to it. Note that it isn't multitenant.
// It's intended to be deployed and scaled alongside the application itself.
func newForwardingHandler(
	lggr logr.Logger,
	dialCtxFunc kedanet.DialContextFunc,
	waitFunc forwardWaitFunc,
	fwdCfg forwardingConfig,
	tlsCfg *tls.Config,
	tracingCfg *config.Tracing,
	placeholderHandler *handler.PlaceholderHandler,
	endpointsCache k8s.EndpointsCache,
) http.Handler {
	transportPool := kedahttp.NewTransportPool(&http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           dialCtxFunc,
		ForceAttemptHTTP2:     fwdCfg.forceAttemptHTTP2,
		MaxIdleConns:          fwdCfg.maxIdleConns,
		MaxIdleConnsPerHost:   fwdCfg.maxIdleConnsPerHost,
		IdleConnTimeout:       fwdCfg.idleConnTimeout,
		TLSHandshakeTimeout:   fwdCfg.tlsHandshakeTimeout,
		ExpectContinueTimeout: fwdCfg.expectContinueTimeout,
		TLSClientConfig:       tlsCfg,
	})

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var uh *handler.Upstream
		ctx := r.Context()
		httpso := util.HTTPSOFromContext(ctx)
		hasFailover := httpso.Spec.ColdStartTimeoutFailoverRef != nil

		// Check if we should serve a placeholder page
		if placeholderHandler != nil && httpso.Spec.PlaceholderConfig != nil && httpso.Spec.PlaceholderConfig.Enabled {
			endpoints, err := endpointsCache.Get(httpso.GetNamespace(), httpso.Spec.ScaleTargetRef.Service)
			if err != nil {
				// Error getting endpoints cache - return 503 Service Unavailable
				lggr.Error(err, "failed to get endpoints from cache while placeholder is configured",
					"namespace", httpso.GetNamespace(),
					"service", httpso.Spec.ScaleTargetRef.Service)
				w.WriteHeader(http.StatusServiceUnavailable)
				if _, writeErr := w.Write([]byte("Service temporarily unavailable - unable to check service status")); writeErr != nil {
					lggr.Error(writeErr, "could not write error response to client")
				}
				return
			}
			
			if workloadActiveEndpoints(endpoints) == 0 {
				if placeholderErr := placeholderHandler.ServePlaceholder(w, r, httpso); placeholderErr != nil {
					lggr.Error(placeholderErr, "failed to serve placeholder page")
					w.WriteHeader(http.StatusBadGateway)
					if _, err := w.Write([]byte("error serving placeholder page")); err != nil {
						lggr.Error(err, "could not write error response to client")
					}
				}
				return
			}
		}

		conditionWaitTimeout := fwdCfg.waitTimeout
		responseHeaderTimeout := fwdCfg.respHeaderTimeout

		if httpso.Spec.Timeouts != nil {
			if httpso.Spec.Timeouts.ConditionWait.Duration > 0 {
				conditionWaitTimeout = httpso.Spec.Timeouts.ConditionWait.Duration
			}

			if httpso.Spec.Timeouts.ResponseHeader.Duration > 0 {
				responseHeaderTimeout = httpso.Spec.Timeouts.ResponseHeader.Duration
			}
		}

		if hasFailover && httpso.Spec.ColdStartTimeoutFailoverRef.TimeoutSeconds > 0 {
			conditionWaitTimeout = time.Duration(httpso.Spec.ColdStartTimeoutFailoverRef.TimeoutSeconds) * time.Second
		}

		waitFuncCtx, done := context.WithTimeout(ctx, conditionWaitTimeout)
		defer done()
		isColdStart, err := waitFunc(
			waitFuncCtx,
			httpso.GetNamespace(),
			httpso.Spec.ScaleTargetRef.Service,
		)
		if err != nil && !hasFailover {
			lggr.Error(err, "wait function failed, not forwarding request")
			w.WriteHeader(http.StatusBadGateway)
			if _, err := fmt.Fprintf(w, "error on backend (%s)", err); err != nil {
				lggr.Error(err, "could not write error response to client")
			}
			return
		}
		if fwdCfg.enableColdStartHeader {
			w.Header().Add("X-KEDA-HTTP-Cold-Start", strconv.FormatBool(isColdStart))
		}
		r.Header.Add("X-KEDA-HTTP-Cold-Start-Ref-Name", httpso.Spec.ScaleTargetRef.Name)
		r.Header.Add("X-KEDA-HTTP-Cold-Start-Ref-Namespace", httpso.Namespace)

		// Get transport from pool based on timeout configuration
		transport := transportPool.Get(responseHeaderTimeout)

		rc := http.NewResponseController(w)
		if err := rc.EnableFullDuplex(); err != nil {
			lggr.Error(err, "Could not enable full duplex on responsewriter, continuing")
		}

		shouldFailover := hasFailover && err != nil
		if tracingCfg.Enabled {
			uh = handler.NewUpstream(otelhttp.NewTransport(transport), tracingCfg, shouldFailover)
		} else {
			uh = handler.NewUpstream(transport, &config.Tracing{}, shouldFailover)
		}
		uh.ServeHTTP(w, r)
	})
}
