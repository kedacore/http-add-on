package routing

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"

	"github.com/go-logr/logr"
)

const GetTablePath = "/routing_ping"

func NewTableHandler(
	lggr logr.Logger,
	table *Table,
) http.Handler {
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
	operatorAdminURL *url.URL,
) (*Table, error) {
	res, err := httpCl.Get(operatorAdminURL.String())
	if err != nil {
		lggr.Error(err, "fetching the routing table URL")
		return nil, err
	}
	defer res.Body.Close()
	newTable := NewTable()
	if err := json.NewDecoder(res.Body).Decode(newTable); err != nil {
		lggr.Error(err, "decoding routing table URL response")
		return nil, err
	}
	lggr.Info("fetched new routing table", "table", newTable.String())
	return newTable, nil
}
