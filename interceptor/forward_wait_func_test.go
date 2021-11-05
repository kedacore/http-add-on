package main

import (
	"context"
	"testing"
	"time"

	"github.com/kedacore/http-add-on/pkg/k8s"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/watch"
)

// Test to make sure the wait function returns a nil error if there is immediately
// one replica on the target deployment
func TestForwardWaitFuncOneReplica(t *testing.T) {
	ctx := context.Background()

	const waitFuncWait = 1 * time.Second
	r := require.New(t)
	const ns = "testNS"
	const deployName = "TestForwardingHandlerDeploy"
	cache := k8s.NewFakeDeploymentCache()
	cache.AddDeployment(*newDeployment(
		ns,
		deployName,
		"myimage",
		[]int32{123},
		nil,
		map[string]string{},
		corev1.PullAlways,
	))

	ctx, done := context.WithTimeout(ctx, waitFuncWait)
	defer done()
	group, ctx := errgroup.WithContext(ctx)

	waitFunc := newDeployReplicasForwardWaitFunc(
		cache,
	)

	group.Go(func() error {
		return waitFunc(ctx, ns, deployName)
	})
	r.NoError(group.Wait(), "wait function failed, but it shouldn't have")
}

// Test to make sure the wait function returns an error if there are no replicas, and that doesn't change
// within a timeout
func TestForwardWaitFuncNoReplicas(t *testing.T) {
	ctx := context.Background()
	const waitFuncWait = 1 * time.Second
	r := require.New(t)
	const ns = "testNS"
	const deployName = "TestForwardingHandlerHoldsDeployment"
	deployment := newDeployment(
		ns,
		deployName,
		"myimage",
		[]int32{123},
		nil,
		map[string]string{},
		corev1.PullAlways,
	)
	deployment.Status.ReadyReplicas = 0
	cache := k8s.NewFakeDeploymentCache()
	cache.AddDeployment(*deployment)

	ctx, done := context.WithTimeout(ctx, waitFuncWait)
	defer done()
	waitFunc := newDeployReplicasForwardWaitFunc(
		cache,
	)

	err := waitFunc(ctx, ns, deployName)
	r.Error(err)
}

func TestWaitFuncWaitsUntilReplicas(t *testing.T) {
	ctx := context.Background()
	r := require.New(t)
	totalWaitDur := 500 * time.Millisecond

	const ns = "testNS"
	const deployName = "TestForwardingHandlerHoldsDeployment"
	deployment := newDeployment(
		ns,
		deployName,
		"myimage",
		[]int32{123},
		nil,
		map[string]string{},
		corev1.PullAlways,
	)
	deployment.Spec.Replicas = k8s.Int32P(0)
	cache := k8s.NewFakeDeploymentCache()
	cache.AddDeployment(*deployment)
	// create a watcher first so that the goroutine
	// can later fetch it and send a message on it
	cache.Watch(ns, deployName)

	ctx, done := context.WithTimeout(ctx, totalWaitDur)
	waitFunc := newDeployReplicasForwardWaitFunc(
		cache,
	)

	// this channel will be closed immediately after the replicas were increased
	replicasIncreasedCh := make(chan struct{})
	go func() {
		time.Sleep(totalWaitDur / 2)
		watcher := cache.GetWatcher(ns, deployName)
		r.NotNil(watcher, "watcher was not found")
		modifiedDeployment := deployment.DeepCopy()
		modifiedDeployment.Spec.Replicas = k8s.Int32P(1)
		watcher.Action(watch.Modified, modifiedDeployment)
		close(replicasIncreasedCh)
	}()
	r.NoError(waitFunc(ctx, ns, deployName))
	done()
}
