package main

import (
	"context"
	"encoding/json"
	"log"
	nethttp "net/http"
	"net/url"

	"github.com/go-logr/logr"
	"github.com/kedacore/http-add-on/pkg/routing"
)

func newRoutingPingHandler(
	lggr logr.Logger,
	operatorAdminURL *url.URL,
	table *routing.Table,
) nethttp.Handler {
	return nethttp.HandlerFunc(func(
		w nethttp.ResponseWriter,
		r *nethttp.Request,
	) {
		newTable, err := fetchRoutingTable(
			r.Context(),
			lggr,
			operatorAdminURL,
		)
		if err != nil {
			log.Printf("error fetching new routing table (%s)", err)
			w.WriteHeader(500)
			w.Write([]byte(
				"error fetching routing table",
			))
			return
		}
		table.Replace(newTable)
		w.WriteHeader(200)
		w.Write([]byte("ok"))
	})
}

func fetchRoutingTable(
	ctx context.Context,
	lggr logr.Logger,
	operatorAdminURL *url.URL,
) (*routing.Table, error) {
	res, err := nethttp.Get(operatorAdminURL.String())
	if err != nil {
		lggr.Error(err, "fetching the routing table URL")
		return nil, err
	}
	defer res.Body.Close()
	newTable := routing.NewTable()
	if err := json.NewDecoder(res.Body).Decode(newTable); err != nil {
		lggr.Error(err, "decoding routing table URL response")
		return nil, err
	}
	lggr.Info("fetched new routing table", "table", newTable.String())
	return newTable, nil
}
