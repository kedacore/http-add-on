package main

import (
	"context"
	"fmt"
	"log"

	"github.com/kedacore/http-add-on/pkg/k8s"
	appsv1 "k8s.io/api/apps/v1"
)

type forwardWaitFunc func(context.Context, string) error

func newDeployReplicasForwardWaitFunc(
	deployCache k8s.DeploymentCache,
) forwardWaitFunc {
	return func(ctx context.Context, deployName string) error {
		deployment, err := deployCache.Get(deployName)
		if err != nil {
			// if we didn't get the initial deployment state, bail out
			return fmt.Errorf("Error getting state for deployment %s (%s)", deployName, err)
		}
		// if there is 1 or more replica, we're done waiting
		if moreThanPtr(deployment.Spec.Replicas, 0) {
			return nil
		}

		watcher := deployCache.Watch(deployName)
		if err != nil {
			return fmt.Errorf("Error getting the stream of deployment changes")
		}
		defer watcher.Stop()
		eventCh := watcher.ResultChan()
		for {
			select {
			case event := <-eventCh:
				deployment, ok := event.Object.(*appsv1.Deployment)
				if !ok {
					log.Println("Didn't get a deployment back in event")
				}
				if moreThanPtr(deployment.Spec.Replicas, 0) {
					return nil
				}
			case <-ctx.Done():
				// otherwise, if we hit the end of the timeout, fail
				return fmt.Errorf(
					"Timeout expired waiting for deployment %s to reach > 0 replicas",
					deployName,
				)
			}
		}
	}
}
