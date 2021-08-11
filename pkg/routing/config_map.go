package routing

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/kedacore/http-add-on/pkg/k8s"
	"github.com/kedacore/http-add-on/pkg/queue"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// the name of the ConfigMap that stores the routing table
	ConfigMapRoutingTableName = "keda-http-routing-table"
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
func FetchTableFromConfigMap(configMap *corev1.ConfigMap, q queue.Counter) (*Table, error) {
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
	return ret, nil
}

// GetTable fetches a table via an RPC call to operatorAdminURL, replaces the
// current value of table with the newly fetched table using table.Replace(), and
// ensures that all hosts are present in q using q.Ensure() for each host in
// the newly fetched table
func GetTable(
	ctx context.Context,
	lggr logr.Logger,
	getter k8s.ConfigMapGetter,
	table *Table,
	q queue.Counter,
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
	newTable, err := FetchTableFromConfigMap(cm, q)
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

	// ensure that every host is in the queue, even if it has
	// zero pending requests. This is important so that the
	// scaler can report on all applications.
	for host := range newTable.m {
		q.Ensure(host)
	}

	// ensure that the queue doesn't have any extra hosts that don't exist in the table
	qCur, err := q.Current()
	if err != nil {
		lggr.Error(
			err,
			"failed to get current queue counts (in order to prune it of missing routing table hosts)",
		)
		return errors.Wrap(err, "failed to get current queue counts")
	}
	for host := range qCur.Counts {
		if _, err := table.Lookup(host); err != nil {
			q.Remove(host)
		}
	}

	return nil
}
