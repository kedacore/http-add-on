package routing

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	"github.com/kedacore/http-add-on/pkg/k8s"
	"github.com/kedacore/http-add-on/pkg/queue"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

// StartConfigMapRoutingTableUpdater starts a loop that does the following:
//
//	- Fetches a full version of the ConfigMap called ConfigMapRoutingTableName in
//	the given namespace ns, and calls table.Replace(newTable) after it does so
//	- Uses watcher to watch for all ADDED or CREATED events on the ConfigMap
//	called ConfigMapRoutingTableName. On either of those events, decodes
//	that ConfigMap into a routing table and stores the new table into table
//	using table.Replace(newTable)
//	- Returns an appropriate non-nil error if ctx.Done() receives
func StartConfigMapRoutingTableUpdater(
	ctx context.Context,
	lggr logr.Logger,
	updateEvery time.Duration,
	getterWatcher k8s.ConfigMapGetterWatcher,
	table *Table,
	q queue.Counter,
) error {
	lggr = lggr.WithName("pkg.routing.StartConfigMapRoutingTableUpdater")
	watchIface, err := getterWatcher.Watch(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}
	defer watchIface.Stop()

	ticker := time.NewTicker(updateEvery)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return errors.Wrap(ctx.Err(), "context is done")
		case <-ticker.C:
			if err := GetTable(ctx, lggr, getterWatcher, table, q); err != nil {
				return errors.Wrap(err, "failed to fetch routing table")
			}

		case evt := <-watchIface.ResultChan():
			evtType := evt.Type
			obj := evt.Object
			if evtType == watch.Added || evtType == watch.Modified {
				cm, ok := obj.(*corev1.ConfigMap)
				// by definition of watchIface, all returned objects should
				// be assertable to a ConfigMap. In the likely impossible
				// case that it isn't, just ignore and move on.
				// This check is just to be defensive.
				if !ok {
					continue
				}
				// the watcher is open on all ConfigMaps in the namespace, so
				// bail out of this loop iteration immediately if the event
				// isn't for the routing table ConfigMap.
				if cm.Name != ConfigMapRoutingTableName {
					continue
				}
				newTable, err := FetchTableFromConfigMap(cm, q)
				if err != nil {
					return err
				}
				table.Replace(newTable)
				if err := updateQueueFromTable(lggr, table, q); err != nil {
					// if we couldn't update the queue, just log but don't bail.
					// we want to give the loop a chance to tick (or receive a new event)
					// and update the table & queue again
					lggr.Error(
						err,
						"failed to update queue from table on ConfigMap change event",
					)
					continue
				}
			}
		}
	}

}
