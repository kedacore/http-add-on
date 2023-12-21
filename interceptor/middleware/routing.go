package middleware

import (
	"fmt"
	"net/http"
	"net/url"
	"regexp"

	"github.com/kedacore/http-add-on/interceptor/handler"
	httpv1alpha1 "github.com/kedacore/http-add-on/operator/apis/http/v1alpha1"
	"github.com/kedacore/http-add-on/pkg/routing"
	"github.com/kedacore/http-add-on/pkg/util"
)

var (
	kpUserAgent = regexp.MustCompile(`(^|\s)kube-probe/`)
	ghUserAgent = regexp.MustCompile(`(^|\s)GoogleHC/`)
)

type Routing struct {
	routingTable    routing.Table
	probeHandler    http.Handler
	upstreamHandler http.Handler
}

func NewRouting(routingTable routing.Table, probeHandler http.Handler, upstreamHandler http.Handler) *Routing {
	return &Routing{
		routingTable:    routingTable,
		probeHandler:    probeHandler,
		upstreamHandler: upstreamHandler,
	}
}

var _ http.Handler = (*Routing)(nil)

func (rm *Routing) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	r = util.RequestWithLoggerWithName(r, "RoutingMiddleware")

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

	stream, err := rm.streamFromHTTPSO(httpso)
	if err != nil {
		sh := handler.NewStatic(http.StatusInternalServerError, err)
		sh.ServeHTTP(w, r)

		return
	}
	r = r.WithContext(util.ContextWithStream(r.Context(), stream))

	rm.upstreamHandler.ServeHTTP(w, r)
}

func (rm *Routing) streamFromHTTPSO(httpso *httpv1alpha1.HTTPScaledObject) (*url.URL, error) {
	//goland:noinspection HttpUrlsUsage
	return url.Parse(fmt.Sprintf(
		"http://%s.%s:%d",
		httpso.Spec.ScaleTargetRef.Service,
		httpso.GetNamespace(),
		httpso.Spec.ScaleTargetRef.Port,
	))
}

func (rm *Routing) isProbe(r *http.Request) bool {
	ua := r.UserAgent()

	return kpUserAgent.Match([]byte(ua)) || ghUserAgent.Match([]byte(ua))
}
