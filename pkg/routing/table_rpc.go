package routing

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"

	"github.com/go-logr/logr"
	"github.com/kedacore/http-add-on/pkg/queue"
)

const (
	routingPingPath  = "/routing_ping"
	routingFetchPath = "/routing_table"
)

// AddFetchRoute adds a route to mux that fetches the current state of table,
// encodes it as JSON, and returns it to the HTTP client
func AddFetchRoute(
	lggr logr.Logger,
	mux *http.ServeMux,
	table *Table,
) {
	lggr = lggr.WithName("pkg.routing.AddFetchRoute")
	lggr.Info("adding routing ping route", "path", routingPingPath)
	mux.Handle(routingFetchPath, newTableHandler(lggr, table))
}

// AddPingRoute adds a route to mux that will accept an empty GET request,
// fetch the current state of the routing table by calling GetTable using fetchURL,
// then returning the new contents of the routing table to the client
func AddPingRoute(
	lggr logr.Logger,
	mux *http.ServeMux,
	httpCl *http.Client,
	fetchURL *url.URL,
	table *Table,
	q queue.Counter,
) {
	lggr = lggr.WithName("pkg.routing.AddPingRoute")
	lggr.Info("adding interceptor routing ping route", "path", routingPingPath)
	mux.HandleFunc(routingPingPath, func(w http.ResponseWriter, r *http.Request) {
		err := GetTable(
			r.Context(),
			httpCl,
			lggr,
			*fetchURL,
			table,
			q,
		)
		if err != nil {
			lggr.Error(err, "fetching new routing table")
			w.WriteHeader(500)
			w.Write([]byte(
				"error fetching routing table",
			))
			return
		}
		w.WriteHeader(200)
		if err := json.NewEncoder(w).Encode(table); err != nil {
			w.WriteHeader(500)
			lggr.Error(err, "writing new routing table to the client")
			return
		}
	})
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

// GetTable fetches a table via an RPC call to operatorAdminURL, replaces the
// current value of table with the newly fetched table using table.Replace(), and
// ensures that all hosts are present in q using q.Ensure() for each host in
// the newly fetched table
func GetTable(
	ctx context.Context,
	httpCl *http.Client,
	lggr logr.Logger,
	operatorAdminURL url.URL,
	table *Table,
	q queue.Counter,
) error {
	lggr = lggr.WithName("pkg.routing.GetTable")

	operatorAdminURL.Path = routingFetchPath

	res, err := httpCl.Get(operatorAdminURL.String())
	if err != nil {
		lggr.Error(
			err,
			"fetching the routing table URL",
			"url",
			operatorAdminURL.String(),
		)
		return err
	}
	defer res.Body.Close()
	newTable := NewTable()
	if err := json.NewDecoder(res.Body).Decode(newTable); err != nil {
		lggr.Error(
			err,
			"decoding routing table URL response",
		)
		return err
	}
	table.Replace(newTable)
	for host := range newTable.m {
		q.Ensure(host)
	}
	return nil
}
