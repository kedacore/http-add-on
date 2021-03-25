package main

import (
	"fmt"
	"log"
	"time"

	"github.com/kedacore/http-add-on/pkg/k8s"
	appsv1 "k8s.io/api/apps/v1"
)

type forwardWaitFunc func() error

func newDeployReplicasForwardWaitFunc(
	deployCache k8s.DeploymentCache,
	deployName string,
	totalWait time.Duration,
) forwardWaitFunc {
	return func() error {
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
		timer := time.NewTimer(totalWait)
		defer timer.Stop()
		for {
			select {
			case event := <-eventCh:
				deployment := event.Object.(*appsv1.Deployment)
				if err != nil {
					log.Printf(
						"Error getting deployment %s after change was triggered (%s)",
						deployName,
						err,
					)
				}
				if moreThanPtr(deployment.Spec.Replicas, 0) {
					return nil
				}
			case <-timer.C:
				// otherwise, if we hit the end of the timeout, fail
				return fmt.Errorf("Timeout expired waiting for deployment %s to reach > 0 replicas", deployName)
			}
		}
	}
}
