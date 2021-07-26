package routing

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"

	"github.com/go-logr/logr"
)

const getTablePath = "/routing_ping"

func AddPingRoute(
	lggr logr.Logger,
	mux *http.ServeMux,
	table *Table,
) {
	lggr = lggr.WithName("pkg.routing.AddPingRoute")
	lggr.Info("adding routing ping route", "path", getTablePath)
	mux.Handle(getTablePath, newTableHandler(lggr, table))
}

func newTableHandler(
	lggr logr.Logger,
	table *Table,
) http.Handler {
	lggr = lggr.WithName("pkg.routing.TableHandler")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := json.NewEncoder(w).Encode(table)
		if err != nil {
			w.WriteHeader(500)
			lggr.Error(err, "encoding logging table JSON")
			w.Write([]byte(
				"error encoding and transmitting the routing table",
			))
		}
	})
}

func GetTable(
	ctx context.Context,
	httpCl *http.Client,
	lggr logr.Logger,
	operatorAdminURL url.URL,
) (*Table, error) {
	lggr = lggr.WithName("pkg.routing.GetTable")

	operatorAdminURL.Path = getTablePath

	res, err := httpCl.Get(operatorAdminURL.String())
	if err != nil {
		lggr.Error(
			err,
			"fetching the routing table URL",
			"url",
			operatorAdminURL.String(),
		)
		return nil, err
	}
	defer res.Body.Close()
	newTable := NewTable()
	if err := json.NewDecoder(res.Body).Decode(newTable); err != nil {
		lggr.Error(
			err,
			"decoding routing table URL response",
		)
		return nil, err
	}
	return newTable, nil
}