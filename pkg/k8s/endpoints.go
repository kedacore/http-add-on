package k8s

import (
	"context"
	"fmt"
	"net/url"

	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func EndpointsForService(
	ctx context.Context,
	cl client.Client,
	ns,
	serviceName,
	servicePort string,
) ([]*url.URL, error) {
	endpoints, err := getEndpoints(ctx, cl, ns, serviceName)
	if err != nil {
		return nil, errors.Wrap(err, "pkg.k8s.EndpointsForService")
	}
	ret := []*url.URL{}
	for _, subset := range endpoints.Subsets {
		for _, addr := range subset.Addresses {
			u, err := url.Parse(
				fmt.Sprintf("http://%s:%s", addr.IP, servicePort),
			)
			if err != nil {
				return nil, err
			}
			ret = append(ret, u)
		}
	}

	return ret, nil
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
