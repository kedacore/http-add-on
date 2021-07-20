package main

import (
	"log"
	"net/http"
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
		newTable, err := routing.GetTable(
			r.Context(),
			http.DefaultClient,
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
