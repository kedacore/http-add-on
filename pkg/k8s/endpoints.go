package k8s

import (
	"context"
	"fmt"
	"net/url"

	discov1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetEndpointsFunc is a type that represents a function that can
// fetch endpoints
type GetEndpointsFunc func(
	ctx context.Context,
	namespace,
	serviceName string,
) (*discov1.EndpointSlice, error)

func EndpointsForService(
	ctx context.Context,
	ns,
	serviceName,
	servicePort string,
	endpointsFn GetEndpointsFunc,
) ([]*url.URL, error) {
	endpointsl, err := endpointsFn(ctx, ns, serviceName)
	if err != nil {
		return nil, fmt.Errorf("pkg.k8s.EndpointsForService: %w", err)
	}
	ret := []*url.URL{}
	for _, endpoint := range endpointsl.Endpoints {
		for _, addr := range endpoint.Addresses {
			u, err := url.Parse(
				fmt.Sprintf("http://%s:%s", addr, servicePort),
			)
			if err != nil {
				return nil, err
			}
			ret = append(ret, u)
		}
	}

	return ret, nil
}

// EndpointsFuncForControllerClient returns a new GetEndpointsFunc
// that uses the controller-runtime client.Client to fetch endpoints
func EndpointsFuncForControllerClient(
	cl client.Client,
) GetEndpointsFunc {
	return func(
		ctx context.Context,
		namespace,
		serviceName string,
	) (*discov1.EndpointSlice, error) {
		ess := &discov1.EndpointSliceList{}

		if err := cl.List(ctx, ess, client.InNamespace(namespace), client.MatchingLabels{discov1.LabelServiceName: serviceName}); err != nil {
			return nil, err
		}
		if len(ess.Items) == 0 {
			return nil, nil
		}
		return &ess.Items[0], nil
	}
}

func EndpointsFuncForK8sClientset(
	cl *kubernetes.Clientset,
) GetEndpointsFunc {
	return func(
		ctx context.Context,
		namespace,
		serviceName string,
	) (*discov1.EndpointSlice, error) {
		endpointSlCl := cl.DiscoveryV1().EndpointSlices(namespace)
		ess, err := endpointSlCl.List(ctx, metav1.ListOptions{LabelSelector: fmt.Sprintf("%s=%s", discov1.LabelServiceName, serviceName)})
		if err != nil {
			return nil, err
		}
		if len(ess.Items) == 0 {
			return nil, nil
		}
		return &ess.Items[0], nil
	}
}
