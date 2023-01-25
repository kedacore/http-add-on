package main

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/kedacore/http-add-on/pkg/k8s"
	appsv1 "k8s.io/api/apps/v1"
)

// forwardWaitFunc is a function that waits for a condition
// before proceeding to serve the request.
type forwardWaitFunc func(context.Context, string, string) (int, error)

func deploymentCanServe(depl appsv1.Deployment) bool {
	return depl.Status.ReadyReplicas > 0
}

func newDeployReplicasForwardWaitFunc(
	lggr logr.Logger,
	deployCache k8s.DeploymentCache,
) forwardWaitFunc {
	return func(ctx context.Context, deployNS, deployName string) (int, error) {
		// get a watcher & its result channel before querying the
		// deployment cache, to ensure we don't miss events
		watcher, err := deployCache.Watch(deployNS, deployName)
		if err != nil {
			return 0, err
		}
		eventCh := watcher.ResultChan()
		defer watcher.Stop()

		deployment, err := deployCache.Get(deployNS, deployName)
		if err != nil {
			// if we didn't get the initial deployment state, bail out
			return 0, fmt.Errorf(
				"error getting state for deployment %s/%s (%s)",
				deployNS,
				deployName,
				err,
			)
		}
		// if there is 1 or more replica, we're done waiting
		if deploymentCanServe(deployment) {
			return int(deployment.Status.ReadyReplicas), nil
		}

		for {
			select {
			case event := <-eventCh:
				deployment, ok := event.Object.(*appsv1.Deployment)
				if !ok {
					lggr.Info(
						"Didn't get a deployment back in event",
					)
				} else if deploymentCanServe(*deployment) {
					return 0, nil
				}
			case <-ctx.Done():
				// otherwise, if the context is marked done before
				// we're done waiting, fail.
				return 0, fmt.Errorf(
					"context marked done while waiting for deployment %s to reach > 0 replicas (%w)",
					deployName,
					ctx.Err(),
				)
			}
		}
	}
}
