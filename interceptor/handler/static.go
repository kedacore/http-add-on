package handler

import (
	"net/http"

	"github.com/kedacore/http-add-on/pkg/k8s"
	"github.com/kedacore/http-add-on/pkg/routing"
	"github.com/kedacore/http-add-on/pkg/util"
)

type Static struct {
	statusCode int
	err        error
}

func NewStatic(statusCode int, err error) *Static {
	return &Static{
		statusCode: statusCode,
		err:        err,
	}
}

var _ http.Handler = (*Static)(nil)

func (sh *Static) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	r = util.RequestWithLoggerWithName(r, "StaticHandler")
	ctx := r.Context()

	logger := util.LoggerFromContext(ctx)
	httpso := util.HTTPSOFromContext(ctx)
	stream := util.StreamFromContext(ctx)

	statusText := http.StatusText(sh.statusCode)
	routingKey := routing.NewKeyFromRequest(r)
	namespacedName := k8s.NamespacedNameFromObject(httpso)
	logger.Error(sh.err, statusText, "routingKey", routingKey, "namespacedName", namespacedName, "stream", stream)

	w.WriteHeader(sh.statusCode)

	//nolint:gosec // G705: statusText is from http.StatusText(), not user input
	if _, err := w.Write([]byte(statusText)); err != nil {
		logger.Error(err, "write failed")
	}
}
