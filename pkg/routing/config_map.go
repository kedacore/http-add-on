package routing

import (
	"context"
	"fmt"
	"github.com/julienschmidt/httprouter"
	nethttp "net/http"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kedacore/http-add-on/pkg/k8s"
	"github.com/kedacore/http-add-on/pkg/queue"
)

const (
	// the name of the ConfigMap that stores the routing table
	ConfigMapRoutingTableName = "keda-http-add-on-routing-table"
	// the key in the ConfigMap data that stores the JSON routing table
	configMapRoutingTableKey = "routing-table"
)

// SaveTableToConfigMap saves the contents of table to the Data field in
// configMap
func SaveTableToConfigMap(table *Table, configMap *corev1.ConfigMap) error {
	tableAsJSON, err := table.MarshalJSON()
	if err != nil {
		return err
	}
	configMap.Data[configMapRoutingTableKey] = string(tableAsJSON)
	return nil
}

// FetchTableFromConfigMap fetches the Data field from configMap, converts it
// to a routing table, and returns it
func FetchTableFromConfigMap(configMap *corev1.ConfigMap, router *httprouter.Router, handler nethttp.Handler) (*Table, error) {
	data, found := configMap.Data[configMapRoutingTableKey]
	if !found {
		return nil, fmt.Errorf(
			"no '%s' key found in the %s ConfigMap",
			configMapRoutingTableKey,
			ConfigMapRoutingTableName,
		)
	}
	ret := NewTable()
	if err := ret.UnmarshalJSON([]byte(data)); err != nil {
		retErr := errors.Wrap(
			err,
			fmt.Sprintf(
				"error decoding '%s' key in %s ConfigMap",
				configMapRoutingTableKey,
				ConfigMapRoutingTableName,
			),
		)
		return nil, retErr
	}
	ret.SyncHostSwitch(router, handler)
	return ret, nil
}

// GetTable fetches the contents of the appropriate ConfigMap that stores
// the routing table, then tries to decode it into a temporary routing table
// data structure.
//
// If that succeeds, it calls table.Replace(newTable), then ensures that
// every host in the routing table exists in the given queue, and no hosts
// exist in the queue that don't exist in the routing table. It uses q.Ensure()
// and q.Remove() to do those things, respectively.
func GetTable(
	ctx context.Context,
	lggr logr.Logger,
	getter k8s.ConfigMapGetter,
	table *Table,
	q queue.Counter,
	router *httprouter.Router,
	handler nethttp.Handler,
) error {
	lggr = lggr.WithName("pkg.routing.GetTable")

	cm, err := getter.Get(
		ctx,
		ConfigMapRoutingTableName,
		metav1.GetOptions{},
	)
	if err != nil {
		lggr.Error(
			err,
			"failed to fetch routing table config map",
			"configMapName",
			ConfigMapRoutingTableName,
		)
		return errors.Wrap(
			err,
			fmt.Sprintf(
				"failed to fetch ConfigMap %s",
				ConfigMapRoutingTableName,
			),
		)
	}
	newTable, err := FetchTableFromConfigMap(cm, router, handler)
	if err != nil {
		lggr.Error(
			err,
			"failed decoding routing table ConfigMap",
			"configMapName",
			ConfigMapRoutingTableName,
		)
		return errors.Wrap(
			err,
			fmt.Sprintf(
				"failed decoding ConfigMap %s into a routing table",
				ConfigMapRoutingTableName,
			),
		)
	}

	table.Replace(newTable)

	return nil
}
