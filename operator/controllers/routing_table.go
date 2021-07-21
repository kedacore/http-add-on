package controllers

import (
	"context"
	"net/http"

	"github.com/go-logr/logr"
	"github.com/kedacore/http-add-on/pkg/routing"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func removeAndUpdateRoutingTable(
	ctx context.Context,
	lggr logr.Logger,
	cl client.Client,
	table *routing.Table,
	host,
	namespace,
	interceptorSvcName,
	interceptorSvcPort string,
) error {
	lggr = lggr.WithName("removeAndUpdateRoutingTable")
	if err := table.RemoveTarget(host); err != nil {
		lggr.Error(
			err,
			"could not remove host from routing table, progressing anyway",
			"host",
			host,
		)
	}
	return pingInterceptors(
		ctx,
		cl,
		http.DefaultClient,
		namespace,
		interceptorSvcName,
		interceptorSvcPort,
	)
}

func addAndUpdateRoutingTable(
	ctx context.Context,
	lggr logr.Logger,
	cl client.Client,
	table *routing.Table,
	host string,
	target routing.Target,
	namespace,
	interceptorSvcName,
	interceptorSvcPort string,
) error {
	lggr = lggr.WithName("addAndUpdateRoutingTable")
	if err := table.AddTarget(host, target); err != nil {
		lggr.Error(
			err,
			"could not add host to routing table, progressing anyway",
			"host",
			host,
		)
	}
	return pingInterceptors(
		ctx,
		cl,
		http.DefaultClient,
		namespace,
		interceptorSvcName,
		interceptorSvcPort,
	)
}
