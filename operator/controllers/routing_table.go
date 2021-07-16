package controllers

import (
	"context"

	"github.com/kedacore/http-add-on/pkg/routing"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func removeAndUpdateRoutingTable(
	ctx context.Context,
	cl client.Client,
	table *routing.Table,
	host,
	namespace,
	interceptorSvcName,
	interceptorSvcPort string,
) error {
	if err := table.RemoveTarget(host); err != nil {
		return err
	}
	return pingInterceptors(
		ctx,
		cl,
		namespace,
		interceptorSvcName,
		interceptorSvcPort,
	)
}

func addAndUpdateRoutingTable(
	ctx context.Context,
	cl client.Client,
	table *routing.Table,
	host string,
	target routing.Target,
	namespace,
	interceptorSvcName,
	interceptorSvcPort string,
) error {
	if err := table.AddTarget(host, target); err != nil {
		return err
	}
	return pingInterceptors(
		ctx,
		cl,
		namespace,
		interceptorSvcName,
		interceptorSvcPort,
	)
}
