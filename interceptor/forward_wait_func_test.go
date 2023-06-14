package main

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"

	"github.com/kedacore/http-add-on/pkg/k8s"
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
		logr.Discard(),
		cache,
	)

	group.Go(func() error {
		_, err := waitFunc(ctx, ns, deployName)
		return err
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
		logr.Discard(),
		cache,
	)

	_, err := waitFunc(ctx, ns, deployName)
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
	_, err := cache.Watch(ns, deployName)
	r.NoError(err)

	ctx, done := context.WithTimeout(ctx, totalWaitDur)
	waitFunc := newDeployReplicasForwardWaitFunc(
		logr.Discard(),
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
	_, err = waitFunc(ctx, ns, deployName)
	r.NoError(err)
	done()
}

// newDeployment creates a new deployment object
// with the given name and the given image. This does not actually create
// the deployment in the cluster, it just creates the deployment object
// in memory
func newDeployment(
	namespace,
	name,
	image string,
	ports []int32,
	env []corev1.EnvVar,
	labels map[string]string,
	pullPolicy corev1.PullPolicy,
) *appsv1.Deployment {
	containerPorts := make([]corev1.ContainerPort, len(ports))
	for i, port := range ports {
		containerPorts[i] = corev1.ContainerPort{
			ContainerPort: port,
		}
	}
	deployment := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind: "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Replicas: k8s.Int32P(1),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Image:           image,
							Name:            name,
							ImagePullPolicy: pullPolicy,
							Ports:           containerPorts,
							Env:             env,
						},
					},
				},
			},
		},
		Status: appsv1.DeploymentStatus{
			ReadyReplicas: 1,
		},
	}

	return deployment
}
