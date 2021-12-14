package routing

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/kedacore/http-add-on/pkg/k8s"
	"github.com/kedacore/http-add-on/pkg/queue"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
	corev1 "k8s.io/api/core/v1"
)

// StartConfigMapRoutingTableUpdater starts a loop that does the following:
//
//	- Fetches a full version of the ConfigMap called ConfigMapRoutingTableName in
//	the given namespace ns, and calls table.Replace(newTable) after it does so
//	- Uses watcher to watch for all ADDED or CREATED events on the ConfigMap
//	called ConfigMapRoutingTableName. On either of those events, decodes
//	that ConfigMap into a routing table and stores the new table into table
//	using table.Replace(newTable)
//	- Execute the callback function, if one exists
//	- Returns an appropriate non-nil error if ctx.Done() receives
func StartConfigMapRoutingTableUpdater(
	ctx context.Context,
	lggr logr.Logger,
	cmInformer *k8s.InformerConfigMapUpdater,
	ns string,
	table *Table,
	q queue.Counter,
	cbFunc func() error,
) error {
	lggr = lggr.WithName("pkg.routing.StartConfigMapRoutingTableUpdater")

	watcher := cmInformer.Watch(ns, ConfigMapRoutingTableName)
	defer watcher.Stop()

	ctx, done := context.WithCancel(ctx)
	defer done()
	grp, ctx := errgroup.WithContext(ctx)

	grp.Go(func() error {
		defer done()
		return cmInformer.Start(ctx)
	})

	grp.Go(func() error {
		defer done()
		for {
			select {
			case event := <-watcher.ResultChan():
				cm, ok := event.Object.(*corev1.ConfigMap)
				// Theoretically this will not happen
				if !ok {
					lggr.Info(
						"The event object observed is not a configmap",
					)
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
				// Execute the callback function, if one exists
				if cbFunc != nil {
					if err := cbFunc(); err != nil {
						lggr.Error(
							err,
							"failed to exec the callback function",
						)
						continue
					}
				}
			case <-ctx.Done():
				return errors.Wrap(ctx.Err(), "context is done")
			}
		}
	})

	if err := grp.Wait(); err != nil {
		lggr.Error(err, "config map routing updater is failed")
		return err
	}
	return nil
}
