package main

import (
	"context"
	"testing"
	"time"

	"github.com/kedacore/http-add-on/pkg/k8s"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/watch"
)

// Test to make sure the wait function returns a nil error if there is immediately
// one replica on the target deployment
func TestForwardWaitFuncOneReplica(t *testing.T) {
	r := require.New(t)
	const ns = "testNS"
	const deployName = "TestForwardingHandlerDeploy"
	cache := k8s.NewMemoryDeploymentCache(map[string]*appsv1.Deployment{
		deployName: k8s.NewDeployment(
			ns,
			deployName,
			"myimage",
			[]int32{123},
			nil,
			map[string]string{},
			"Always",
		),
	})
	waitFunc := newDeployReplicasForwardWaitFunc(
		cache,
		deployName,
		1*time.Second,
	)

	ctx, done := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer done()
	group, ctx := errgroup.WithContext(ctx)
	group.Go(waitFunc)
	r.NoError(group.Wait())
}

// Test to make sure the wait function returns an error if there are no replicas, and that doesn't change
// within a timeout
func TestForwardWaitFuncNoReplicas(t *testing.T) {
	r := require.New(t)
	const ns = "testNS"
	const deployName = "TestForwardingHandlerHoldsDeployment"
	deployment := k8s.NewDeployment(
		ns,
		deployName,
		"myimage",
		[]int32{123},
		nil,
		map[string]string{},
		"Always",
	)
	deployment.Spec.Replicas = k8s.Int32P(0)
	cache := k8s.NewMemoryDeploymentCache(map[string]*appsv1.Deployment{
		deployName: deployment,
	})

	const timeout = 200 * time.Millisecond
	waitFunc := newDeployReplicasForwardWaitFunc(
		cache,
		deployName,
		1*time.Second,
	)

	err := waitFunc()
	r.Error(err)
}

func TestWaitFuncWaitsUntilReplicas(t *testing.T) {
	r := require.New(t)
	totalWaitDur := 500 * time.Millisecond

	const ns = "testNS"
	const deployName = "TestForwardingHandlerHoldsDeployment"
	deployment := k8s.NewDeployment(
		ns,
		deployName,
		"myimage",
		[]int32{123},
		nil,
		map[string]string{},
		"Always",
	)
	deployment.Spec.Replicas = k8s.Int32P(0)
	cache := k8s.NewMemoryDeploymentCache(map[string]*appsv1.Deployment{
		deployName: deployment,
	})
	waitFunc := newDeployReplicasForwardWaitFunc(cache, deployName, totalWaitDur)
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
	r.NoError(waitFunc())
}
