package main

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
	discov1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"

	"github.com/kedacore/http-add-on/pkg/k8s"
)

// Test to make sure the wait function returns a nil error if there is immediately
// one active endpoint on the target deployment
func TestForwardWaitFuncOneReplica(t *testing.T) {
	ctx := context.Background()

	const waitFuncWait = 1 * time.Second
	r := require.New(t)
	const ns = "testNS"
	const endpointsName = "TestForwardingHandler"
	endpoints := *newEndpoint(ns, endpointsName)
	cache := k8s.NewFakeEndpointsCache()
	cache.Set(endpoints)
	r.NoError(cache.SetEndpoints(ns, endpointsName, 1))

	ctx, done := context.WithTimeout(ctx, waitFuncWait)
	defer done()
	group, ctx := errgroup.WithContext(ctx)

	waitFunc := newWorkloadReplicasForwardWaitFunc(
		logr.Discard(),
		cache,
	)

	group.Go(func() error {
		_, err := waitFunc(ctx, ns, endpointsName)
		return err
	})
	r.NoError(group.Wait(), "wait function failed, but it shouldn't have")
}

// Test to make sure the wait function returns an error if there are active endpoints, and that doesn't change
// within a timeout
func TestForwardWaitFuncNoReplicas(t *testing.T) {
	ctx := context.Background()
	const waitFuncWait = 1 * time.Second
	r := require.New(t)
	const ns = "testNS"
	const endpointsName = "TestForwardWaitFuncNoReplicas"
	endpoints := *newEndpoint(ns, endpointsName)
	cache := k8s.NewFakeEndpointsCache()
	cache.Set(endpoints)

	ctx, done := context.WithTimeout(ctx, waitFuncWait)
	defer done()
	waitFunc := newWorkloadReplicasForwardWaitFunc(
		logr.Discard(),
		cache,
	)

	_, err := waitFunc(ctx, ns, endpointsName)
	r.Error(err)
}

func TestWaitFuncWaitsUntilReplicas(t *testing.T) {
	ctx := context.Background()
	r := require.New(t)
	totalWaitDur := 500 * time.Millisecond

	const ns = "testNS"
	const endpointsName = "TestForwardingHandlerHolds"

	endpoints := *newEndpoint(ns, endpointsName)
	cache := k8s.NewFakeEndpointsCache()
	cache.Set(endpoints)
	// create a watcher first so that the goroutine
	// can later fetch it and send a message on it
	_, err := cache.Watch(ns, endpointsName)
	r.NoError(err)

	ctx, done := context.WithTimeout(ctx, totalWaitDur)
	waitFunc := newWorkloadReplicasForwardWaitFunc(
		logr.Discard(),
		cache,
	)

	// this channel will be closed immediately after the active endpoints were increased
	activeEndpointsIncreasedCh := make(chan struct{})
	go func() {
		time.Sleep(totalWaitDur / 2)
		watcher := cache.GetWatcher(ns, endpointsName)
		r.NotNil(watcher, "watcher was not found")
		modifiedEndpoints := endpoints.DeepCopy()
		modifiedEndpoints.Endpoints = []discov1.Endpoint{
			{
				Addresses: []string{
					"1.2.3.4",
				},
			},
		}
		watcher.Action(watch.Modified, modifiedEndpoints)
		close(activeEndpointsIncreasedCh)
	}()
	_, err = waitFunc(ctx, ns, endpointsName)
	r.NoError(err)
	done()
}

// newEndpoint creates a new endpoints object
// with the given name and the given image. This does not actually create
// the endpoints in the cluster, it just creates the endpoints object
// in memory
func newEndpoint(
	namespace,
	name string,
) *discov1.EndpointSlice {
	endpoints := &discov1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: name,
			Namespace:    namespace,
			Labels: map[string]string{
				discov1.LabelServiceName: name,
			},
		},
	}

	return endpoints
}
