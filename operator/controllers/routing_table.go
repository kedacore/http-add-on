package controllers

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/kedacore/http-add-on/pkg/k8s"
	"github.com/kedacore/http-add-on/pkg/routing"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const routingTableName = "keda-http-routing-table"

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

	return updateRoutingMap(ctx, cl, namespace, table, lggr)
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
	return updateRoutingMap(ctx, cl, namespace, table, lggr)
}

func updateRoutingMap(
	ctx context.Context,
	cl client.Client,
	namespace string,
	table *routing.Table,
	lggr logr.Logger,
) error {
	tableAsJSON, marshalErr := table.MarshalJSON()
	if marshalErr != nil { // should never happen
		return marshalErr
	}

	routingConfigMap, err := k8s.GetConfigMap(ctx, cl, namespace, routingTableName)
	if err != nil { return err }

	if routingConfigMap == nil { // if the routing table doesn't exist, we need to create it with the latest data
		routingTableData := map[string]string{
			"routing-table": string(tableAsJSON),
		}

		routingTableLabels := map[string]string{
			"control-plane": "operator",
			"keda.sh/addon": "http-add-on",
			"app": "http-add-on",
			"name": "http-add-on-routing-table",
		}

		if err := k8s.CreateConfigMap(ctx, cl, k8s.NewConfigMap(namespace, routingTableName, routingTableLabels, routingTableData), lggr); err != nil {
			return err
		}
	} else {
		newRoutingTable := routingConfigMap.DeepCopy()
		newRoutingTable.Data["routing-table"] = string(tableAsJSON)
		if _, patchErr := k8s.PatchConfigMap(ctx, cl, lggr, routingConfigMap, newRoutingTable); patchErr != nil { return patchErr }
	}

	return nil
}
