package controllers

import (
	"context"
	"fmt"
	"net/http"

	"github.com/kedacore/http-add-on/pkg/k8s"
	"golang.org/x/sync/errgroup"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func pingInterceptors(
	ctx context.Context,
	cl client.Client,
	ns,
	interceptorSvcName,
	interceptorPort string,
) error {
	endpoints, err := getEndpoints(ctx, cl, ns, interceptorSvcName)
	if err != nil {
		return err
	}
	endpointURLs, err := k8s.EndpointsForService(
		ctx,
		endpoints,
		interceptorSvcName,
		interceptorPort,
	)
	if err != nil {
		return err
	}
	errGrp, _ := errgroup.WithContext(ctx)
	for _, endpointURL := range endpointURLs {
		endpointStr := endpointURL.String()
		errGrp.Go(func() error {
			fullAddr := fmt.Sprintf("%s/routing_ping", endpointStr)
			_, err := http.Get(fullAddr)
			return err
		})
	}
	return errGrp.Wait()
}

func getEndpoints(
	ctx context.Context,
	cl client.Client,
	ns,
	interceptorSvcName string,
) (*v1.Endpoints, error) {
	endpts := &v1.Endpoints{}
	if err := cl.Get(ctx, client.ObjectKey{
		Namespace: ns,
		Name:      interceptorSvcName,
	}, endpts); err != nil {
		return nil, err
	}
	return endpts, nil
}
