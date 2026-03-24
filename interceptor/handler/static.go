package handler

import (
	"net/http"

	"github.com/kedacore/http-add-on/pkg/k8s"
	"github.com/kedacore/http-add-on/pkg/routing"
	"github.com/kedacore/http-add-on/pkg/util"
)

func NewStatic(statusCode int, err error) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r = util.RequestWithLoggerWithName(r, "StaticHandler")
		ctx := r.Context()

		logger := util.LoggerFromContext(ctx)
		ir := util.InterceptorRouteFromContext(ctx)
		stream := util.UpstreamURLFromContext(ctx)

		statusText := http.StatusText(statusCode)
		routingKey := routing.NewKeyFromRequest(r)
		namespacedName := k8s.NamespacedNameFromObject(ir)
		logger.Error(err, statusText, "routingKey", routingKey, "namespacedName", namespacedName, "stream", stream)

		http.Error(w, statusText, statusCode)
	})
}
