package main

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	discov1 "k8s.io/api/discovery/v1"

	"github.com/kedacore/http-add-on/pkg/k8s"
)

// forwardWaitFunc is a function that waits for a condition
// before proceeding to serve the request.
type forwardWaitFunc func(context.Context, string, string) (bool, error)

func workloadActiveEndpoints(endpoints discov1.EndpointSlice) int {
	total := 0
	for _, e := range endpoints.Endpoints {
		total += len(e.Addresses)
	}
	return total
}

func newWorkloadReplicasForwardWaitFunc(
	lggr logr.Logger,
	endpointCache k8s.EndpointsCache,
) forwardWaitFunc {
	return func(ctx context.Context, endpointNS, endpointName string) (bool, error) {
		// get a watcher & its result channel before querying the
		// endpoints cache, to ensure we don't miss events
		watcher, err := endpointCache.Watch(endpointNS, endpointName)
		if err != nil {
			return false, err
		}
		eventCh := watcher.ResultChan()
		defer watcher.Stop()

		endpoints, err := endpointCache.Get(endpointNS, endpointName)
		if err != nil {
			// if we didn't get the initial endpoints state, bail out
			return false, fmt.Errorf(
				"error getting state for endpoints %s/%s: %w",
				endpointNS,
				endpointName,
				err,
			)
		}
		// if there is 1 or more active endpoints, we're done waiting
		activeEndpoints := workloadActiveEndpoints(endpoints)
		if activeEndpoints > 0 {
			return false, nil
		}

		for {
			select {
			case event := <-eventCh:
				endpoints, ok := event.Object.(*discov1.EndpointSlice)
				if !ok {
					lggr.Info(
						"Didn't get a endpoints back in event",
					)
				} else if activeEndpoints := workloadActiveEndpoints(*endpoints); activeEndpoints > 0 {
					return true, nil
				}
			case <-ctx.Done():
				// otherwise, if the context is marked done before
				// we're done waiting, fail.
				return false, fmt.Errorf(
					"context marked done while waiting for workload reach > 0 replicas: %w",
					ctx.Err(),
				)
			}
		}
	}
}
