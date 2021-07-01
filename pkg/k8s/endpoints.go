package k8s

import (
	"context"
	"fmt"
	"net/url"

	corev1 "k8s.io/api/core/v1"
)

func EndpointsForService(
	ctx context.Context,
	endpoints *corev1.Endpoints,
	serviceName string,
	servicePort string,
) ([]*url.URL, error) {
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
