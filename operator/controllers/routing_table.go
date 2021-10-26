package controllers

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/kedacore/http-add-on/pkg/k8s"
	"github.com/kedacore/http-add-on/pkg/routing"
	pkgerrs "github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
	if err != nil && !errors.IsNotFound(err) {
		// if there is an error other than not found on the ConfigMap, we should
		// fail
		lggr.Error(
			err,
			"other issue fetching the routing table ConfigMap",
			"configMapName",
			routing.ConfigMapRoutingTableName,
		)
		return pkgerrs.Wrap(err, "routing table ConfigMap fetch error")
	} else if errors.IsNotFound(err) || routingConfigMap == nil {
		// if either the routing table ConfigMap doesn't exist or for some reason it's
		// nil in memory, we need to create it
		lggr.Info(
			"routing table ConfigMap didn't exist, creating it",
			"configMapName",
			routing.ConfigMapRoutingTableName,
		)
		routingTableLabels := map[string]string{
			"control-plane": "operator",
			"keda.sh/addon": "http-add-on",
			"app":           "http-add-on",
			"name":          "http-add-on-routing-table",
		}
		cm := k8s.NewConfigMap(
			namespace,
			routing.ConfigMapRoutingTableName,
			routingTableLabels,
			map[string]string{},
		)
		if err := routing.SaveTableToConfigMap(table, cm); err != nil {
			return err
		}
		if err := k8s.CreateConfigMap(
			ctx,
			lggr,
			cl,
			cm,
		); err != nil {
			return err
		}
	} else {
		newCM := routingConfigMap.DeepCopy()
		if err := routing.SaveTableToConfigMap(table, newCM); err != nil {
			return err
		}
		if _, patchErr := k8s.PatchConfigMap(ctx, lggr, cl, routingConfigMap, newCM); patchErr != nil {
			return patchErr
		}
	}

	return nil
}
