package k8s

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	k8sfake "k8s.io/client-go/kubernetes/fake"
)

func TestK8DeploymentCacheGet(t *testing.T) {
	r := require.New(t)
	ctx, done := context.WithCancel(context.Background())
	defer done()

	const ns = "testns"
	const name = "testdepl"
	expectedDepl := newDeployment(
		ns,
		name,
		"testing",
		nil,
		nil,
		make(map[string]string),
		core.PullAlways,
	)
	fakeClientset := k8sfake.NewSimpleClientset(expectedDepl)
	fakeApps := fakeClientset.AppsV1()

	cache, err := NewK8sDeploymentCache(
		ctx,
		logr.Discard(),
		fakeApps.Deployments(ns),
	)
	r.NoError(err)

	depl, err := cache.Get(name)
	r.NoError(err)
	r.Equal(name, depl.ObjectMeta.Name)

	noneRet, err := cache.Get("noexist")
	r.NotNil(err)
	// note: the returned deployment will be empty, not nil,
	// because this function doesn't return a pointer. so,
	// we have to check some of the fields inside the deployment
	// to make sure they're empty
	r.Nil(noneRet.Spec.Replicas)
	r.Empty(noneRet.ObjectMeta.Name)

}

func TestK8sDeploymentCacheMergeAndBroadcastList(t *testing.T) {
	r := require.New(t)
	ctx, done := context.WithCancel(
		context.Background(),
	)
	defer done()
	lggr := logr.Discard()
	listerWatcher := newFakeDeploymentListerWatcher()
	cache, err := NewK8sDeploymentCache(
		ctx,
		lggr,
		listerWatcher,
	)
	r.NoError(err)
	deplList := &appsv1.DeploymentList{
		Items: []appsv1.Deployment{
			*newDeployment("testns", "testadded", "testing", nil, nil, nil, core.PullAlways),
		},
	}
	checkAllEvents := map[int]struct{}{}
	for i := range deplList.Items {
		checkAllEvents[i] = struct{}{}
	}
	checkNoEvents := map[int]struct{}{}

	// first call merge with no deployments in the cache.
	// all events should be ADDED
	evts := callMergeAndBcast(t, cache, deplList, checkAllEvents)
	r.Equal(len(deplList.Items), len(evts))
	for i := 0; i < len(deplList.Items); i++ {
		evt := evts[i]
		// all events should be ADDED events because nothing
		// was in the deployment cache before merge was called
		r.Equal(watch.Added, evt.Type)
		depl, ok := evt.Object.(*appsv1.Deployment)
		if !ok {
			t.Fatal("event came through with no deployment")
		}
		r.Equal(deplList.Items[i].Spec, depl.Spec)
	}

	// now call merge with deployments in the cache.
	// since nothing has changed, there should be no returned events
	evts = callMergeAndBcast(t, cache, deplList, checkNoEvents)
	r.Equal(0, len(evts))

	// now change one of the existing deployments. there should be 1
	// MODIFIED event
	newList := *deplList
	replicas := *newList.Items[0].Spec.Replicas + 1
	newList.Items[0].Spec.Replicas = &replicas
	evts = callMergeAndBcast(t, cache, &newList, checkAllEvents)
	r.Equal(len(deplList.Items), len(evts))
	for i := 0; i < len(deplList.Items); i++ {
		evt := evts[i]
		r.Equal(
			watch.Modified,
			evt.Type,
			"wrong event type for deployment # %d (%v)",
			i,
			evt.Object,
		)
		depl, ok := evt.Object.(*appsv1.Deployment)
		if !ok {
			t.Fatal("event came through with no deployment")
		}
		r.Equal(deplList.Items[i].Spec, depl.Spec)
	}
}

// callMergeAndBcast calls mergeAndBroadcast on cache for each
// deployment in deplList. Then, for each "true" value at the
// corresponding index in the shouldEvent slice, it tries
// to receive an event. it then puts the event in the returned map.
// the key for the event is the same index.
func callMergeAndBcast(
	t *testing.T,
	cache *K8sDeploymentCache,
	deplList *appsv1.DeploymentList,
	shouldEvent map[int]struct{},
) map[int]watch.Event {
	t.Helper()
	var wg sync.WaitGroup
	evtsLock := new(sync.Mutex)
	evts := map[int]watch.Event{}
	for i, depl := range deplList.Items {
		_, checkEvent := shouldEvent[i]
		if checkEvent {
			wg.Add(1)
			// make sure to register the watcher
			// before starting the goroutine. this way
			// we're guaranteed that we're registered
			// with the internal broadcaster
			// before we call mergeAndBroadcastList
			// below
			watcher := cache.Watch(depl.GetName())
			go func(idx int, watcher watch.Interface) {
				defer wg.Done()
				defer watcher.Stop()
				watchCh := watcher.ResultChan()
				tmr := time.NewTimer(1 * time.Second)
				defer tmr.Stop()
				select {
				case <-tmr.C:
					t.Error("timeout waiting for event")
				case evt := <-watchCh:
					evtsLock.Lock()
					evts[idx] = evt
					evtsLock.Unlock()
				}
			}(i, watcher)
		}
	}

	// call mergeAndBroadcast after starting up all the
	// receiver goroutines so that mergeAndBroadcast can
	// broadcast to all of them. if this is run before
	// all the receivers, there may not be any watchers
	// registered to the broadcaster.
	wg.Add(1)
	go func() {
		defer wg.Done()
		cache.mergeAndBroadcastList(deplList)
	}()

	// after we know all events were received,
	// make sure we got the ones we expected
	wg.Wait()
	return evts
}

func TestK8sDeploymentCacheAddEvt(t *testing.T) {
	r := require.New(t)
	ctx, done := context.WithCancel(
		context.Background(),
	)
	defer done()
	cache, err := NewK8sDeploymentCache(ctx, logr.Discard(), newFakeDeploymentListerWatcher())
	r.NoError(err)
	checkDeploymentsInCache := func(depls ...appsv1.Deployment) {
		t.Helper()
		r := require.New(t)
		for _, depl := range depls {
			ret, ok := cache.latest[depl.GetName()]
			r.True(ok)
			r.Equal(depl, ret)
		}
	}

	// add a first deployment and make sure it exists in
	// the latest cache
	depl1 := newDeployment("testns", "testdepl1", "testing", nil, nil, nil, core.PullAlways)
	evt1 := watch.Event{
		Type:   watch.Added, // doesn't matter, addEvt doesn't look at this
		Object: depl1,
	}
	r.NoError(cache.addEvt(evt1))
	r.Equal(1, len(cache.latest))
	checkDeploymentsInCache(*depl1)

	// add a second (different name) and make sure both exist
	// in the cache
	depl2 := *depl1
	depl2.Name = "testdepl2"
	evt2 := watch.Event{
		Type:   watch.Modified,
		Object: &depl2,
	}
	r.NoError(cache.addEvt(evt2))
	r.Equal(2, len(cache.latest))
	checkDeploymentsInCache(*depl1, depl2)

	// try to add a pod, make sure addEvt fails, and afterward, make sure
	// the original 2 deployments are still in the cache
	otherObj := &core.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "somepod"},
	}
	evt3 := watch.Event{
		Type:   watch.Added,
		Object: otherObj,
	}
	r.Error(cache.addEvt(evt3))
	r.Equal(2, len(cache.latest))
	checkDeploymentsInCache(*depl1, depl2)
}

// test to make sure that, even when no events come through, the
// update loop eventually fetches the latest state of deployments
func TestK8sDeploymentCachePeriodicFetch(t *testing.T) {
	r := require.New(t)
	ctx, done := context.WithCancel(
		context.Background(),
	)
	defer done()
	lw := newFakeDeploymentListerWatcher()
	cache, err := NewK8sDeploymentCache(ctx, logr.Discard(), lw)
	r.NoError(err)
	const tickDur = 10 * time.Millisecond
	go cache.StartWatcher(ctx, logr.Discard(), tickDur)
	depl := newDeployment("testns", "testdepl", "testing", nil, nil, nil, core.PullAlways)
	// add the deployment without sending an event, to make sure that
	// the internal loop won't receive any events and will rely on
	// just the ticker
	lw.addDeployment(*depl)
	time.Sleep(tickDur * 2)
	// make sure that the deployment was fetched
	fetched, err := cache.Get(depl.ObjectMeta.Name)
	r.NoError(err)
	r.Equal(*depl, fetched)
	r.Equal(0, len(lw.row.getEvents()))
}

func signalSent(name string, sig <-chan struct{}) error {
	const dur = 100 * time.Millisecond
	select {
	case <-time.After(dur):
		return fmt.Errorf("%s signal not called within %s", name, dur)
	case <-sig:
	}
	return nil
}

func errSignalSent(name string, sig <-chan error) error {
	const dur = 100 * time.Millisecond
	select {
	case <-time.After(dur):
		return fmt.Errorf("%s signal not called within %s", name, dur)
	case <-sig:
	}
	return nil
}

// test to make sure that the update loop tries to
// re-establish watch streams when they're broken
func TestK8sDeploymentCacheRewatch(t *testing.T) {
	r := require.New(t)
	ctx, done := context.WithCancel(
		context.Background(),
	)
	defer done()
	lw := newFakeDeploymentListerWatcher()
	// channel that will receive when watch is called
	watchCh := make(chan struct{})
	watchCallNum := 0
	lw.watchCB = func() {
		select {
		case watchCh <- struct{}{}:
		case <-time.After(100 * time.Millisecond):
			t.Fatalf("watch callback called but not acknowledged within 100ms for call num %d", watchCallNum+1)
		}
		watchCallNum++
	}
	cache, err := NewK8sDeploymentCache(ctx, logr.Discard(), lw)
	r.NoError(err)

	// start up the cache watcher with a very long tick duration,
	// to ensure that the only way it will get updates is from the
	// watch stream
	const tickDur = 1000 * time.Second
	watcherErrCh := make(chan error)
	go func() {
		watcherErrCh <- cache.StartWatcher(ctx, logr.Discard(), tickDur)
	}()

	// make sure the Watch() was called and that the watcher
	// itself didn't error
	r.NoError(signalSent("watch", watchCh))
	r.Error(errSignalSent("watcher error", watcherErrCh))

	// close all open watch watch channels, then make sure Watch()
	// was called again and there was no watch error
	lw.row.Stop()
	r.NoError(signalSent("watch", watchCh))
	r.Error(errSignalSent("watcher error", watcherErrCh))

	// add the deployment and send an event.
	depl := newDeployment("testns", "testdepl", "testing", nil, nil, nil, core.PullAlways)
	r.NoError(lw.addDeploymentAndSendEvent(*depl, 100*time.Millisecond))

	// make sure that watch wasn't called a third time
	// and that the watcher itself didn't error
	r.Error(signalSent("watch", watchCh))
	r.Error(errSignalSent("watcher error", watcherErrCh))

	// make sure that an event came through
	r.Equal(1, len(lw.row.getEvents()))
	// make sure that the deployment was fetched
	fetched, err := cache.Get(depl.ObjectMeta.Name)
	r.NoError(err)
	r.Equal(*depl, fetched)
}

// test to make sure that when the context is closed, the deployment
// cache stops
func TestK8sDeploymentCacheStopped(t *testing.T) {
	r := require.New(t)
	ctx, done := context.WithCancel(context.Background())

	fakeClientset := k8sfake.NewSimpleClientset()
	fakeApps := fakeClientset.AppsV1()

	cache, err := NewK8sDeploymentCache(
		ctx,
		logr.Discard(),
		fakeApps.Deployments("doesn't matter"),
	)
	r.NoError(err)

	done()
	err = cache.StartWatcher(ctx, logr.Discard(), time.Millisecond)
	r.Error(err, "deployment cache watcher didn't return an error")
	r.True(errors.Is(err, context.Canceled), "expected a context cancel error")
}

func TestK8sDeploymentCacheBasicWatch(t *testing.T) {
	r := require.New(t)
	ctx, done := context.WithCancel(
		context.Background(),
	)
	defer done()

	const ns = "testns"
	const name = "testdepl"
	expectedDepl := newDeployment(
		ns,
		name,
		"testing",
		nil,
		nil,
		make(map[string]string),
		core.PullAlways,
	)
	fakeClientset := k8sfake.NewSimpleClientset()
	fakeDeployments := fakeClientset.AppsV1().Deployments(ns)

	cache, err := NewK8sDeploymentCache(
		ctx,
		logr.Discard(),
		fakeDeployments,
	)
	r.NoError(err)
	go cache.StartWatcher(ctx, logr.Discard(), time.Millisecond)

	watcher := cache.Watch(name)
	defer watcher.Stop()

	createSentCh := make(chan struct{})
	createErrCh := make(chan error)
	go func() {
		time.Sleep(200 * time.Millisecond)
		_, err := fakeDeployments.Create(
			ctx,
			expectedDepl,
			metav1.CreateOptions{},
		)
		if err != nil {
			createErrCh <- err
		} else {
			close(createSentCh)
		}
	}()

	// first make sure that the send happened, and there was
	// no error
	select {
	case <-createSentCh:
	case err := <-createErrCh:
		r.NoError(err, "error creating the new deployment to trigger the event")
	case <-time.After(400 * time.Millisecond):
		r.Fail("the create operation didn't happen after 400 ms")
	}

	// then make sure that the deployment was actually
	// received
	select {
	case obj := <-watcher.ResultChan():
		depl, ok := obj.Object.(*appsv1.Deployment)
		r.True(ok, "expected a deployment but got a %#V", obj)
		r.Equal(ns, depl.ObjectMeta.Namespace)
		r.Equal(name, depl.ObjectMeta.Name)
	case <-time.After(500 * time.Millisecond):
		r.Fail("didn't get a watch event after 500 ms")
	}
}
