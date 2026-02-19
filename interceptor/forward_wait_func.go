package main

import (
	"context"

	"github.com/kedacore/http-add-on/pkg/k8s"
)

// forwardWaitFunc is a function that waits for a condition
// before proceeding to serve the request.
type forwardWaitFunc func(context.Context, string, string) (bool, error)

func newWorkloadReplicasForwardWaitFunc(
	readyCache *k8s.ReadyEndpointsCache,
) forwardWaitFunc {
	return func(ctx context.Context, endpointNS, endpointName string) (bool, error) {
		return readyCache.WaitForReady(ctx, endpointNS+"/"+endpointName)
	}
}
