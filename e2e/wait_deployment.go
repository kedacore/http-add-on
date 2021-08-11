package e2e

import (
	"context"
	"fmt"
	"time"

	"github.com/kedacore/http-add-on/pkg/k8s"
	"golang.org/x/sync/errgroup"
	appsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func waitUntilDeployment(
	ctx context.Context,
	cl client.Client,
	dur time.Duration,
	ns,
	name string,
	fn func(context.Context, *appsv1.Deployment) error,
) error {
	depl := &appsv1.Deployment{}
	if err := cl.Get(
		ctx,
		k8s.ObjKey(ns, name),
		depl,
	); err != nil {
		return err
	}
	return waitUntil(ctx, dur, func(ctx context.Context) error {
		return fn(ctx, depl)
	})
}

func waitUntilDeploymentAvailable(
	ctx context.Context,
	cl client.Client,
	dur time.Duration,
	ns,
	name string,
) error {
	return waitUntilDeployment(
		ctx,
		cl,
		dur,
		ns,
		name,
		func(ctx context.Context, depl *appsv1.Deployment) error {
			if depl.Status.UnavailableReplicas == 0 {
				return nil
			}
			return fmt.Errorf(
				"deployment %s still has %d unavailable replicas",
				"keda-operator",
				depl.Status.UnavailableReplicas,
			)
		},
	)
}

func waitUntilDeplomentsAvailable(
	ctx context.Context,
	cl client.Client,
	dur time.Duration,
	ns string,
	names []string,
) error {
	ctx, done := context.WithTimeout(ctx, dur)
	defer done()
	g, ctx := errgroup.WithContext(ctx)
	for _, name := range names {
		n := name
		g.Go(func() error {
			return waitUntilDeploymentAvailable(
				ctx,
				cl,
				dur,
				ns,
				n,
			)
		})
	}
	return g.Wait()
}
