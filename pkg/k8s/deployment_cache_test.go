package k8s

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	none, err := cache.Get(name + "noexist")
	r.NotNil(err)
	r.Nil(none)
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
	lw.addDeployment(*depl, false)
	time.Sleep(tickDur * 2)
	// make sure that the deployment was fetched
	fetched, err := cache.Get(depl.ObjectMeta.Name)
	r.NoError(err)
	r.Equal(*depl, *fetched)
	r.Equal(0, len(lw.getRecorder().Events()))
}

// test to make sure that the update loop tries to re-establish watch
// streams when they're broken
func TestK8sDeploymentCacheRewatch(t *testing.T) {
	r := require.New(t)
	ctx, done := context.WithCancel(
		context.Background(),
	)
	defer done()
	lw := newFakeDeploymentListerWatcher()
	cache, err := NewK8sDeploymentCache(ctx, logr.Discard(), lw)
	r.NoError(err)
	const tickDur = 10 * time.Millisecond

	watcherErrCh := make(chan error)
	go func() {
		watcherErrCh <- cache.StartWatcher(ctx, logr.Discard(), tickDur)
	}()

	// close all open watch channels, but allow them to be reopened. this should
	// trigger the cache to reopen a stream
	lw.getFakeWatcher().closeOpenChans(true)

	// wait 1/2 second to make sure the watcher doesn't return any errors
	select {
	case err := <-watcherErrCh:
		r.NoError(err)
	case <-time.After(500 * time.Millisecond):
	}

	// now ensure that the watcher has re-established the watch and
	// can receive events

	depl := newDeployment("testns", "testdepl", "testing", nil, nil, nil, core.PullAlways)
	// add the deployment without sending an event, to make sure that
	// the internal loop won't receive any events and will rely on
	// just the ticker
	lw.addDeployment(*depl, true)
	// make sure that the deployment was fetched
	fetched, err := cache.Get(depl.ObjectMeta.Name)
	r.NoError(err)
	r.Equal(*depl, *fetched)
	r.Equal(1, len(lw.getRecorder().Events()))
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
