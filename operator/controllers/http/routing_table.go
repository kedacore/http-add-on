package http

import (
	"context"

	"github.com/go-logr/logr"
	pkgerrs "github.com/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kedacore/http-add-on/pkg/k8s"
	"github.com/kedacore/http-add-on/pkg/routing"
)

func removeAndUpdateRoutingTable(
	ctx context.Context,
	lggr logr.Logger,
	cl client.Client,
	table *routing.Table,
	host,
	namespace string,
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

	return updateRoutingMap(ctx, lggr, cl, namespace, table)
}

func addAndUpdateRoutingTable(
	ctx context.Context,
	lggr logr.Logger,
	cl client.Client,
	table *routing.Table,
	host string,
	target routing.Target,
	namespace string,
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
	return updateRoutingMap(ctx, lggr, cl, namespace, table)
}

func updateRoutingMap(
	ctx context.Context,
	lggr logr.Logger,
	cl client.Client,
	namespace string,
	table *routing.Table,
) error {
	lggr = lggr.WithName("updateRoutingMap")
	routingConfigMap, err := k8s.GetConfigMap(ctx, cl, namespace, routing.ConfigMapRoutingTableName)
	if err != nil {
		lggr.Error(err, "Error getting configmap", "configMapName", routing.ConfigMapRoutingTableName)
		return pkgerrs.Wrap(err, "routing table ConfigMap fetch error")
	}
	newCM := routingConfigMap.DeepCopy()
	if err := routing.SaveTableToConfigMap(table, newCM); err != nil {
		lggr.Error(err, "couldn't save new routing table to ConfigMap", "configMap", routing.ConfigMapRoutingTableName)
		return pkgerrs.Wrap(err, "ConfigMap save error")
	}
	if _, err := k8s.PatchConfigMap(ctx, lggr, cl, routingConfigMap, newCM); err != nil {
		lggr.Error(err, "couldn't save new routing table ConfigMap to Kubernetes", "configMap", routing.ConfigMapRoutingTableName)
		return pkgerrs.Wrap(err, "saving routing table ConfigMap to Kubernetes")
	}

	return nil
}
