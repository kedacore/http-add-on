package main

import (
	"context"
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
	"github.com/kedacore/http-add-on/pkg/util"
)

type forwardingConfig struct {
	waitTimeout           time.Duration
	respHeaderTimeout     time.Duration
	enableColdStartHeader bool
}

func newForwardingConfigFromTimeouts(t config.Timeouts, s config.Serving) forwardingConfig {
	return forwardingConfig{
		waitTimeout:           t.WorkloadReplicas,
		respHeaderTimeout:     t.ResponseHeader,
		enableColdStartHeader: s.EnableColdStartHeader,
	}
}

// newForwardingHandler takes in the service URL for the app backend
// and forwards incoming requests to it. Note that it isn't multitenant.
// It's intended to be deployed and scaled alongside the application itself.
func newForwardingHandler(
	lggr logr.Logger,
	baseTransport *http.Transport,
	waitFunc forwardWaitFunc,
	fwdCfg forwardingConfig,
	tracingCfg config.Tracing,
) http.Handler {
	transportPool := kedahttp.NewTransportPool(baseTransport)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var uh *handler.Upstream
		ctx := r.Context()
		ir := util.InterceptorRouteFromContext(ctx)
		hasFallback := ir.Spec.ColdStart != nil && ir.Spec.ColdStart.Fallback != nil

		conditionWaitTimeout := fwdCfg.waitTimeout
		responseHeaderTimeout := fwdCfg.respHeaderTimeout

		// TODO(v1): replace temporary annotation-based timeout overrides with proper config when we know which timeouts we want to keep
		if v, ok := ir.Annotations[k8s.AnnotationConditionWaitTimeout]; ok {
			if d, err := time.ParseDuration(v); err == nil && d > 0 {
				conditionWaitTimeout = d
			}
		}
		if v, ok := ir.Annotations[k8s.AnnotationResponseHeaderTimeout]; ok {
			if d, err := time.ParseDuration(v); err == nil && d > 0 {
				responseHeaderTimeout = d
			}
		}

		waitFuncCtx, done := context.WithTimeout(ctx, conditionWaitTimeout)
		defer done()
		isColdStart, err := waitFunc(
			waitFuncCtx,
			ir.Namespace,
			ir.Spec.Target.Service,
		)
		if err != nil && !hasFallback {
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

		// Get transport from pool based on timeout configuration
		transport := transportPool.Get(responseHeaderTimeout)

		rc := http.NewResponseController(w)
		if err := rc.EnableFullDuplex(); err != nil {
			lggr.Error(err, "Could not enable full duplex on responsewriter, continuing")
		}

		shouldFallback := hasFallback && err != nil
		if tracingCfg.Enabled {
			uh = handler.NewUpstream(otelhttp.NewTransport(transport), tracingCfg, shouldFallback)
		} else {
			uh = handler.NewUpstream(transport, config.Tracing{}, shouldFallback)
		}
		uh.ServeHTTP(w, r)
	})
}
