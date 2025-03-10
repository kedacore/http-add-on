package middleware

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"

	"github.com/kedacore/http-add-on/interceptor/handler"
	httpv1alpha1 "github.com/kedacore/http-add-on/operator/apis/http/v1alpha1"
	"github.com/kedacore/http-add-on/pkg/k8s"
	"github.com/kedacore/http-add-on/pkg/routing"
	"github.com/kedacore/http-add-on/pkg/util"
)

var (
	kubernetesProbeUserAgent = regexp.MustCompile(`(^|\s)kube-probe/`)
	googleHCUserAgent        = regexp.MustCompile(`(^|\s)GoogleHC/`)
	awsELBserAgent           = regexp.MustCompile(`(^|\s)ELB-HealthChecker/`)
)

type Routing struct {
	routingTable    routing.Table
	probeHandler    http.Handler
	upstreamHandler http.Handler
	svcCache        k8s.ServiceCache
	tlsEnabled      bool
}

func NewRouting(routingTable routing.Table, probeHandler http.Handler, upstreamHandler http.Handler, svcCache k8s.ServiceCache, tlsEnabled bool) *Routing {
	return &Routing{
		routingTable:    routingTable,
		probeHandler:    probeHandler,
		upstreamHandler: upstreamHandler,
		svcCache:        svcCache,
		tlsEnabled:      tlsEnabled,
	}
}

var _ http.Handler = (*Routing)(nil)

func (rm *Routing) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	r = util.RequestWithLoggerWithName(r, "RoutingMiddleware")

	// logger := util.LoggerFromContext(r.Context())
	httpso := rm.routingTable.Route(r)

	if httpso == nil {
		if rm.isProbe(r) {
			rm.probeHandler.ServeHTTP(w, r)
			return
		}

		sh := handler.NewStatic(http.StatusNotFound, nil)
		sh.ServeHTTP(w, r)

		return
	}
	r = r.WithContext(util.ContextWithHTTPSO(r.Context(), httpso))

	stream, err := rm.streamFromHTTPSO(r.Context(), httpso)
	if err != nil {
		sh := handler.NewStatic(http.StatusInternalServerError, err)
		sh.ServeHTTP(w, r)

		return
	}
	r = r.WithContext(util.ContextWithStream(r.Context(), stream))

	rm.upstreamHandler.ServeHTTP(w, r)
}

func (rm *Routing) getPort(ctx context.Context, httpso *httpv1alpha1.HTTPScaledObject) (int32, error) {
	if httpso.Spec.ScaleTargetRef.Port != 0 {
		return httpso.Spec.ScaleTargetRef.Port, nil
	}
	if httpso.Spec.ScaleTargetRef.PortName == "" {
		return 0, fmt.Errorf(`must specify either "port" or "portName"`)
	}
	svc, err := rm.svcCache.Get(ctx, httpso.GetNamespace(), httpso.Spec.ScaleTargetRef.Service)
	if err != nil {
		return 0, fmt.Errorf("failed to get Service: %w", err)
	}
	for _, port := range svc.Spec.Ports {
		if port.Name == httpso.Spec.ScaleTargetRef.PortName {
			return port.Port, nil
		}
	}
	return 0, fmt.Errorf("portName %q not found in Service", httpso.Spec.ScaleTargetRef.PortName)
}

func (rm *Routing) streamFromHTTPSO(ctx context.Context, httpso *httpv1alpha1.HTTPScaledObject) (*url.URL, error) {
	port, err := rm.getPort(ctx, httpso)
	if err != nil {
		return nil, fmt.Errorf("failed to get port: %w", err)
	}
	if rm.tlsEnabled {
		return url.Parse(fmt.Sprintf(
			"https://%s.%s:%d",
			httpso.Spec.ScaleTargetRef.Service,
			httpso.GetNamespace(),
			port,
		))
	}
	//goland:noinspection HttpUrlsUsage
	return url.Parse(fmt.Sprintf(
		"http://%s.%s:%d",
		httpso.Spec.ScaleTargetRef.Service,
		httpso.GetNamespace(),
		port,
	))
}

func (rm *Routing) isProbe(r *http.Request) bool {
	ua := r.UserAgent()

	return kubernetesProbeUserAgent.Match([]byte(ua)) || googleHCUserAgent.Match([]byte(ua)) || awsELBserAgent.Match([]byte(ua))
}
