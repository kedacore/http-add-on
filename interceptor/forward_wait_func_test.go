package main

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/require"
	discov1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kedacore/http-add-on/pkg/k8s"
)

func TestForwardWaitFuncOneReplica(t *testing.T) {
	ctx := context.Background()

	const waitFuncWait = 1 * time.Second
	r := require.New(t)
	const ns = "testNS"
	const endpointsName = "TestForwardingHandler"

	readyCache := k8s.NewReadyEndpointsCache(logr.Discard())
	readyCache.Update(ns+"/"+endpointsName, []*discov1.EndpointSlice{
		newEndpoint(ns, endpointsName, "1.2.3.4"),
	})

	ctx, done := context.WithTimeout(ctx, waitFuncWait)
	defer done()

	waitFunc := newWorkloadReplicasForwardWaitFunc(readyCache)
	isColdStart, err := waitFunc(ctx, ns, endpointsName)
	r.NoError(err, "wait function failed, but it shouldn't have")
	r.False(isColdStart, "should not be a cold start when endpoints are already ready")
}

func TestForwardWaitFuncNoReplicas(t *testing.T) {
	ctx := context.Background()
	const waitFuncWait = 100 * time.Millisecond
	r := require.New(t)
	const ns = "testNS"
	const endpointsName = "TestForwardWaitFuncNoReplicas"

	readyCache := k8s.NewReadyEndpointsCache(logr.Discard())

	ctx, done := context.WithTimeout(ctx, waitFuncWait)
	defer done()
	waitFunc := newWorkloadReplicasForwardWaitFunc(readyCache)

	_, err := waitFunc(ctx, ns, endpointsName)
	r.Error(err)
}

func TestWaitFuncWaitsUntilReplicas(t *testing.T) {
	ctx := context.Background()
	r := require.New(t)
	totalWaitDur := 500 * time.Millisecond

	const ns = "testNS"
	const endpointsName = "TestForwardingHandlerHolds"

	readyCache := k8s.NewReadyEndpointsCache(logr.Discard())

	ctx, done := context.WithTimeout(ctx, totalWaitDur)
	defer done()
	waitFunc := newWorkloadReplicasForwardWaitFunc(readyCache)

	go func() {
		time.Sleep(totalWaitDur / 2)
		readyCache.Update(ns+"/"+endpointsName, []*discov1.EndpointSlice{
			newEndpoint(ns, endpointsName, "1.2.3.4"),
		})
	}()

	isColdStart, err := waitFunc(ctx, ns, endpointsName)
	r.NoError(err)
	r.True(isColdStart, "should be a cold start when we had to wait for endpoints")
}

func newEndpoint(namespace, name string, addresses ...string) *discov1.EndpointSlice {
	endpoints := make([]discov1.Endpoint, 0, len(addresses))
	for _, addr := range addresses {
		endpoints = append(endpoints, discov1.Endpoint{
			Addresses: []string{addr},
		})
	}
	return &discov1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name + "-slice",
			Namespace: namespace,
			Labels: map[string]string{
				discov1.LabelServiceName: name,
			},
		},
		Endpoints: endpoints,
	}
}
