package main

import (
	"context"
	"testing"
	"time"

	"github.com/kedacore/http-add-on/pkg/k8s"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
	appsv1 "k8s.io/api/apps/v1"
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
	cache := k8s.NewMemoryDeploymentCache(map[string]appsv1.Deployment{
		deployName: *newDeployment(
			ns,
			deployName,
			"myimage",
			[]int32{123},
			nil,
			map[string]string{},
			corev1.PullAlways,
		),
	})

	ctx, done := context.WithTimeout(ctx, waitFuncWait)
	defer done()
	group, ctx := errgroup.WithContext(ctx)

	waitFunc := newDeployReplicasForwardWaitFunc(
		cache,
	)

	group.Go(func() error {
		return waitFunc(ctx, deployName)
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
	cache := k8s.NewMemoryDeploymentCache(map[string]appsv1.Deployment{
		deployName: *deployment,
	})

	ctx, done := context.WithTimeout(ctx, waitFuncWait)
	defer done()
	waitFunc := newDeployReplicasForwardWaitFunc(
		cache,
	)

	err := waitFunc(ctx, deployName)
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
	cache := k8s.NewMemoryDeploymentCache(map[string]appsv1.Deployment{
		deployName: *deployment,
	})
	ctx, done := context.WithTimeout(ctx, totalWaitDur)
	defer done()
	waitFunc := newDeployReplicasForwardWaitFunc(
		cache,
	)
	// this channel will be closed immediately after the replicas were increased
	replicasIncreasedCh := make(chan struct{})
	go func() {
		time.Sleep(totalWaitDur / 2)
		cache.RWM.RLock()
		defer cache.RWM.RUnlock()
		watcher := cache.Watchers[deployName]
		modifiedDeployment := deployment.DeepCopy()
		modifiedDeployment.Spec.Replicas = k8s.Int32P(1)
		watcher.Action(watch.Modified, modifiedDeployment)
		close(replicasIncreasedCh)
	}()
	r.NoError(waitFunc(ctx, deployName))
}
