package admin

import (
	"context"
	"fmt"
	"net/http"

	"github.com/go-logr/logr"
	"github.com/kedacore/http-add-on/pkg/routing"
)

func StartServer(
	ctx context.Context,
	logger logr.Logger,
	port int,
	table *routing.Table,
) error {
	mux := http.NewServeMux()
	mux.Handle("/routing", newRoutingTableHandler(logger, table))
	addrStr := fmt.Sprintf(":%d", port)
	return http.ListenAndServe(
		addrStr,
		mux,
	)
}
