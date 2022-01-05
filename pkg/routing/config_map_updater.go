package routing

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/kedacore/http-add-on/pkg/k8s"
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
				newTable, err := FetchTableFromConfigMap(cm)
				if err != nil {
					return err
				}
				table.Replace(newTable)
			}
		}
	})

	if err := grp.Wait(); err != nil {
		lggr.Error(err, "config map routing updater is failed")
		return err
	}
	return nil
}
