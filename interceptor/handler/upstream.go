package handler

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httputil"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kedacore/http-add-on/interceptor/config"
	httpv1beta1 "github.com/kedacore/http-add-on/operator/apis/http/v1beta1"
	kedahttp "github.com/kedacore/http-add-on/pkg/http"
	"github.com/kedacore/http-add-on/pkg/util"
)

const appProtocolH2C = "kubernetes.io/h2c"

var (
	bufferPool = newBufferPool()

	errNilUpstreamURL = errors.New("upstream URL is nil")
)

type Upstream struct {
	defaultTransportPool   *kedahttp.TransportPool
	http2OnlyTransportPool *kedahttp.TransportPool
	reader                 client.Reader
	responseHeaderTimeout  time.Duration
	tracingCfg             config.Tracing
}

func NewUpstream(baseTransport *http.Transport, reader client.Reader, tracingCfg config.Tracing, responseHeaderTimeout time.Duration) *Upstream {
	if baseTransport == nil {
		panic("baseTransport must not be nil")
	}
	if reader == nil {
		panic("reader must not be nil")
	}
	defaultTransport, http2OnlyTransport := kedahttp.NewTransports(baseTransport)

	return &Upstream{
		defaultTransportPool:   kedahttp.NewTransportPool(defaultTransport),
		http2OnlyTransportPool: kedahttp.NewTransportPool(http2OnlyTransport),
		reader:                 reader,
		responseHeaderTimeout:  responseHeaderTimeout,
		tracingCfg:             tracingCfg,
	}
}

var _ http.Handler = (*Upstream)(nil)

func (uh *Upstream) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	r = util.RequestWithLoggerWithName(r, "UpstreamHandler")
	ctx := r.Context()

	if uh.tracingCfg.Enabled {
		span := trace.SpanFromContext(ctx)
		span.SetAttributes(
			attribute.String("service", "keda-http-interceptor-proxy-upstream"),
			attribute.String("cold-start", w.Header().Get(kedahttp.HeaderColdStart)),
		)
	}

	url := util.UpstreamURLFromContext(ctx)

	if url == nil {
		sh := NewStatic(http.StatusInternalServerError, errNilUpstreamURL)
		sh.ServeHTTP(w, r)

		return
	}

	// Select transport with per-route or global response header timeout.
	ir := util.InterceptorRouteFromContext(ctx)
	responseHeaderTimeout := uh.responseHeaderTimeout
	if ir != nil {
		if ir.Spec.Timeouts.ResponseHeader != nil {
			responseHeaderTimeout = ir.Spec.Timeouts.ResponseHeader.Duration
		}
	}

	pool := uh.defaultTransportPool
	if uh.requiresHTTP2(ctx, ir) {
		pool = uh.http2OnlyTransportPool
	}
	transport := pool.Get(responseHeaderTimeout)

	var rt http.RoundTripper = transport
	if uh.tracingCfg.Enabled {
		rt = otelhttp.NewTransport(transport)
	}

	rc := http.NewResponseController(w)
	if err := rc.EnableFullDuplex(); err != nil {
		util.LoggerFromContext(ctx).Error(err, "could not enable full duplex on response writer, continuing")
	}

	// Close the request body to prevent a server panic when it tries to read
	// the next keep-alive request after EnableFullDuplex (golang/go#68560).
	if r.Body != nil {
		defer func() { _ = r.Body.Close() }()
	}

	proxy := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(url)
			// Preserve original Host header (SetURL rewrites it by default).
			pr.Out.Host = pr.In.Host

			// Preserve and extend X-Forwarded-... headers from upstream proxies
			pr.Out.Header["X-Forwarded-For"] = pr.In.Header["X-Forwarded-For"]
			pr.SetXForwarded()
			if host := pr.In.Header.Get("X-Forwarded-Host"); host != "" {
				pr.Out.Header.Set("X-Forwarded-Host", host)
			}
			if proto := pr.In.Header.Get("X-Forwarded-Proto"); proto != "" {
				pr.Out.Header.Set("X-Forwarded-Proto", proto)
			}
		},
		BufferPool: bufferPool,
		Transport:  rt,
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			code := http.StatusBadGateway
			var netErr net.Error
			if errors.As(err, &netErr) && netErr.Timeout() {
				// Respond with 504 Gateway Timeout on timeouts to differentiate from general server errors
				code = http.StatusGatewayTimeout
			} else if errors.Is(err, context.DeadlineExceeded) {
				code = http.StatusGatewayTimeout
			}
			sh := NewStatic(code, err)
			sh.ServeHTTP(w, r)
		},
	}

	proxy.ServeHTTP(w, r)
}

func (uh *Upstream) requiresHTTP2(ctx context.Context, ir *httpv1beta1.InterceptorRoute) bool {
	if ir == nil {
		return false
	}

	var svc corev1.Service
	err := uh.reader.Get(ctx, types.NamespacedName{
		Namespace: ir.Namespace,
		Name:      ir.Spec.Target.Service,
	}, &svc)
	if err != nil {
		util.LoggerFromContext(ctx).Error(err, "failed to look up Service for appProtocol, using default transport",
			"service", ir.Spec.Target.Service,
			"namespace", ir.Namespace,
		)
		return false
	}

	for _, port := range svc.Spec.Ports {
		if !matchesTargetPort(ir.Spec.Target, port) {
			continue
		}
		return port.AppProtocol != nil && *port.AppProtocol == appProtocolH2C
	}
	return false
}

func matchesTargetPort(target httpv1beta1.TargetRef, port corev1.ServicePort) bool {
	if target.Port != 0 {
		return port.Port == target.Port
	}
	return port.Name == target.PortName
}
