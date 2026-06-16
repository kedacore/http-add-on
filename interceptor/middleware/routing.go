package middleware

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kedacore/http-add-on/interceptor/handler"
	httpv1beta "github.com/kedacore/http-add-on/operator/apis/http/v1beta1"
	"github.com/kedacore/http-add-on/pkg/routing"
	"github.com/kedacore/http-add-on/pkg/util"
)

type Routing struct {
	routingTable   routing.Table
	next           http.Handler
	reader         client.Reader
	tlsEnabled     bool
	requestTimeout time.Duration
}

func NewRouting(next http.Handler, routingTable routing.Table, reader client.Reader, tlsEnabled bool, requestTimeout time.Duration) *Routing {
	return &Routing{
		routingTable:   routingTable,
		next:           next,
		reader:         reader,
		tlsEnabled:     tlsEnabled,
		requestTimeout: requestTimeout,
	}
}

var _ http.Handler = (*Routing)(nil)

func (rm *Routing) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ir := rm.routingTable.Route(r)
	if ir == nil {
		sh := handler.NewStatic(http.StatusNotFound, nil)
		sh.ServeHTTP(w, r)

		return
	}

	// Populate route identity for metric labels.
	if info := routeInfoFromContext(r.Context()); info != nil {
		info.Name = ir.Name
		info.Namespace = ir.Namespace
	}

	url, err := rm.resolveUpstreamURL(r.Context(), ir.Spec.Target.AsServiceRef(), ir.Namespace)
	if err != nil {
		sh := handler.NewStatic(http.StatusInternalServerError, err)
		sh.ServeHTTP(w, r)

		return
	}

	// Batch context operations to reduce allocations in the happy path
	ctx := r.Context()
	logger := util.LoggerFromContext(ctx)
	ctx = util.ContextWithLogger(ctx, logger.WithName("RoutingMiddleware"))
	ctx = util.ContextWithInterceptorRoute(ctx, ir)
	ctx = util.ContextWithUpstreamURL(ctx, url)
	// Capture the SNI hostname before EndpointResolver may rewrite the URL to a
	// pod IP. Empty for non-TLS upstreams.
	serverName := ""
	if rm.tlsEnabled {
		serverName = url.Hostname()
	}
	ctx = util.ContextWithUpstreamServerName(ctx, serverName)
	portName := rm.resolveUpstreamPortName(ctx, ir.Spec.Target.AsServiceRef(), ir.Namespace)
	ctx = util.ContextWithUpstreamPortName(ctx, portName)

	if ir.Spec.ColdStart != nil && ir.Spec.ColdStart.Fallback != nil && ir.Spec.ColdStart.Fallback.Service != nil {
		fallbackURL, err := rm.resolveUpstreamURL(ctx, *ir.Spec.ColdStart.Fallback.Service, ir.Namespace)
		if err != nil {
			sh := handler.NewStatic(http.StatusInternalServerError, err)
			sh.ServeHTTP(w, r)
			return
		}

		ctx = util.ContextWithFallbackURL(ctx, fallbackURL)
	}

	// Apply per-route or global request deadline
	requestTimeout := rm.requestTimeout
	if ir.Spec.Timeouts.Request != nil {
		requestTimeout = ir.Spec.Timeouts.Request.Duration
	}
	if requestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, requestTimeout)
		defer cancel()
	}

	r = r.WithContext(ctx)
	rm.next.ServeHTTP(w, r)
}

// resolveUpstreamPortName returns the named port for direct-pod routing, or ""
// when the port is unnamed or the Service lookup fails ("" matches unnamed
// EndpointSlice ports).
func (rm *Routing) resolveUpstreamPortName(ctx context.Context, svc httpv1beta.ServiceRef, namespace string) string {
	if svc.PortName != "" {
		return svc.PortName
	}

	var k8sSvc corev1.Service
	if err := rm.reader.Get(ctx, types.NamespacedName{Namespace: namespace, Name: svc.Name}, &k8sSvc); err != nil {
		util.LoggerFromContext(ctx).V(1).Info(
			"failed to look up Service for upstream port name; direct-pod routing may not engage for this request",
			"namespace", namespace, "service", svc.Name, "err", err.Error(),
		)
		return ""
	}

	for _, port := range k8sSvc.Spec.Ports {
		if port.Port == svc.Port {
			return port.Name
		}
	}
	return ""
}

func (rm *Routing) resolvePort(ctx context.Context, svc httpv1beta.ServiceRef, namespace string) (int32, error) {
	if svc.Port != 0 {
		return svc.Port, nil
	}
	if svc.PortName == "" {
		return 0, fmt.Errorf(`must specify either "port" or "portName"`)
	}

	var k8sSvc corev1.Service
	err := rm.reader.Get(ctx, types.NamespacedName{Namespace: namespace, Name: svc.Name}, &k8sSvc)
	if err != nil {
		return 0, fmt.Errorf("failed to get Service: %w", err)
	}

	for _, port := range k8sSvc.Spec.Ports {
		if svc.PortName == port.Name {
			return port.Port, nil
		}
	}
	return 0, fmt.Errorf("port name %q not found in Service", svc.PortName)
}

func (rm *Routing) resolveUpstreamURL(ctx context.Context, svc httpv1beta.ServiceRef, namespace string) (*url.URL, error) {
	port, err := rm.resolvePort(ctx, svc, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to get port: %w", err)
	}

	scheme := "http"
	if rm.tlsEnabled {
		scheme = "https"
	}

	return &url.URL{
		Scheme: scheme,
		Host:   fmt.Sprintf("%s.%s:%d", svc.Name, namespace, port),
	}, nil
}
