package middleware

import (
	"net/http"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kedacore/http-add-on/pkg/k8s"
	"github.com/kedacore/http-add-on/pkg/util"
)

// Placeholder short-circuits requests with a static response when the
// backend has no ready endpoints and a placeholder response is configured.
// It sits before the EndpointResolver so the caller gets an immediate
// reply instead of blocking until the backend scales up.
type Placeholder struct {
	next       http.Handler
	readyCache *k8s.ReadyEndpointsCache
	reader     client.Reader
}

// NewPlaceholder returns a middleware that serves a static placeholder
// response when the target has no ready endpoints. The reader is used
// to resolve response bodies stored in ConfigMaps.
func NewPlaceholder(next http.Handler, readyCache *k8s.ReadyEndpointsCache, reader client.Reader) *Placeholder {
	return &Placeholder{
		next:       next,
		readyCache: readyCache,
		reader:     reader,
	}
}

var _ http.Handler = (*Placeholder)(nil)

func (p *Placeholder) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ir := util.InterceptorRouteFromContext(r.Context())

	if ir.Spec.ColdStart != nil && ir.Spec.ColdStart.Placeholder != nil && ir.Spec.ColdStart.Placeholder.Response != nil {
		serviceKey := ir.Namespace + "/" + ir.Spec.Target.Service
		if !p.readyCache.HasReadyEndpoints(serviceKey) {
			serveStaticResponse(w, r, p.reader, ir, ir.Spec.ColdStart.Placeholder.Response, http.StatusServiceUnavailable)
			return
		}
	}

	p.next.ServeHTTP(w, r)
}
