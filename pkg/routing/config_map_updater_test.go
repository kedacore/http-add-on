package routing

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/kedacore/http-add-on/pkg/k8s"
	"github.com/kedacore/http-add-on/pkg/queue"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes/fake"
	clgotesting "k8s.io/client-go/testing"
)

// fake adapters for the k8s.GetterWatcher interface.
//
// Note that there is another way to fake the k8s getter and
// watcher types.
//
// we could use the "fake" package in k8s.io/client-go
// (https://pkg.go.dev/k8s.io/client-go@v0.22.0/kubernetes/fake)
// instead of creating and using these structs, but doing so
// requires internal knowledge of several layers of the client-go
// module, since it's not well documented (even if it were,
// you would need to touch a few different packages to get it
// working).
//
// I've (arschles) chosen to create these structs and sidestep
// the entire process, since this approach is explicit and only
// requires knowledge of the k8s.GetterWatcher interface in this
// codebase, the standard k8s/client-go package (which you
// already need to know to understand this codebase), and the
// fake watcher, which you would need to understand using either
// approach. The fake watcher documentation is linked below:
//
// (https://pkg.go.dev/k8s.io/apimachinery@v0.21.3/pkg/watch#NewFake),
type fakeCMGetterWatcher struct {
	k8s.ConfigMapGetter
	k8s.ConfigMapWatcher
}

type fakeConfigMapWatcher struct {
	watchIface watch.Interface
}

func (c fakeConfigMapWatcher) Watch(
	ctx context.Context,
	opts metav1.ListOptions,
) (watch.Interface, error) {
	return c.watchIface, nil
}

func TestStartUpdateLoop(t *testing.T) {
	r := require.New(t)
	a := assert.New(t)
	lggr := logr.Discard()
	ctx, done := context.WithCancel(context.Background())
	// ensure that we call done so that we clean
	// up running test resources like the update loop, etc...
	defer done()
	const (
		interval = 10 * time.Millisecond
		ns       = "testns"
	)

	q := queue.NewFakeCounter()
	table := NewTable()
	table.AddTarget("host1", NewTarget(
		"svc1",
		8080,
		"depl1",
		100,
	))

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ConfigMapRoutingTableName,
			Namespace: ns,
		},
		Data: map[string]string{},
	}
	r.NoError(SaveTableToConfigMap(table, cm))

	fakeWatcher := watch.NewFake()
	fakeGetter := fake.NewSimpleClientset(cm)
	getterWatcher := fakeCMGetterWatcher{
		ConfigMapGetter: fakeGetter.
			CoreV1().
			ConfigMaps(ns),
		ConfigMapWatcher: fakeConfigMapWatcher{fakeWatcher},
	}
	defer fakeWatcher.Stop()

	grp, ctx := errgroup.WithContext(ctx)

	grp.Go(func() error {
		err := StartConfigMapRoutingTableUpdater(
			ctx,
			lggr,
			interval,
			getterWatcher,
			table,
			q,
		)
		// we purposefully cancel the context below,
		// so we need to ignore that error.
		if !errors.Is(err, context.Canceled) {
			return err
		}
		return nil
	})

	// send a watch event in parallel. we'll ensure that it
	// made it through in the below loop
	grp.Go(func() error {
		fakeWatcher.Add(cm)
		return nil
	})

	cmGetActions := []clgotesting.Action{}
	otherGetActions := []clgotesting.Action{}
	const waitDur = interval * 5
	time.Sleep(waitDur)

	for _, action := range fakeGetter.Actions() {
		verb := action.GetVerb()
		resource := action.GetResource().Resource
		// record, then ignore all actions that were not for
		// ConfigMaps.
		// the loop should not do anything with other resources
		if resource != "configmaps" {
			otherGetActions = append(otherGetActions, action)
			continue
		} else if verb == "get" {
			cmGetActions = append(cmGetActions, action)
		}
	}

	// assert (don't require) these conditions so that
	// we can check them, fail if necessary, but continue onward
	// to check the result of the error group afterward
	a.Equal(
		0,
		len(otherGetActions),
		"unexpected actions on non-ConfigMap resources: %s",
		otherGetActions,
	)
	a.Greater(
		len(cmGetActions),
		0,
		"no get actions for ConfigMaps",
	)

	done()
	// if this test returns without timing out,
	// then we can be sure that the fakeWatcher was
	// able to send a watch event. if that times out
	// or otherwise fails, the update loop was not properly
	// listening for these events.
	r.NoError(grp.Wait())

	// ensure that the queue and table host lists matches
	// exactly
	table.l.RLock()
	curTable := table.m
	curQCounts, err := q.Current()
	r.NoError(err)
	// check that the queue has every host in the table
	for tableHost := range curTable {
		_, ok := curQCounts.Counts[tableHost]
		r.True(
			ok,
			"host %s not found in queue",
			tableHost,
		)
	}
	// check that the table has every host in the queue
	for qHost := range curQCounts.Counts {
		_, ok := curTable[qHost]
		r.True(
			ok,
			"host %s not found in table",
			qHost,
		)
	}

}
