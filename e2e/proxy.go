package e2e

import (
	"context"
	"fmt"

	"golang.org/x/sync/errgroup"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/flowcontrol"
)

func makeRequestsToSvc(ctx context.Context, cfg *rest.Config, ns, svcName string, svcPort, numReqs int) error {
	cls, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return err
	}
	restCl := cls.CoreV1().RESTClient()
	makeReq := func(ctx context.Context) error {
		req := restCl.
			Get().
			Namespace(ns).
			Resource("services").
			Name(fmt.Sprintf("%s:%d", svcName, svcPort)).
			SubResource("proxy").
			Throttle(flowcontrol.NewFakeAlwaysRateLimiter())
		res := req.Do(ctx)
		return res.Error()
	}
	g, ctx := errgroup.WithContext(ctx)
	for i := 0; i < numReqs; i++ {
		g.Go(func() error {
			return makeReq(ctx)
		})
	}
	return g.Wait()
}
