package k8s

import (
	"context"
	"fmt"
	"net/url"

	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
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
) (*v1.Endpoints, error)

func EndpointsForService(
	ctx context.Context,
	ns,
	serviceName,
	servicePort string,
	endpointsFn GetEndpointsFunc,
) ([]*url.URL, error) {
	endpoints, err := endpointsFn(ctx, ns, serviceName)
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

// EndpointsFuncForControllerClient returns a new GetEndpointsFunc
// that uses the controller-runtime client.Client to fetch endpoints
func EndpointsFuncForControllerClient(
	cl client.Client,
) GetEndpointsFunc {
	return func(
		ctx context.Context,
		namespace,
		serviceName string,
	) (*v1.Endpoints, error) {
		endpts := &v1.Endpoints{}
		if err := cl.Get(ctx, client.ObjectKey{
			Namespace: namespace,
			Name:      serviceName,
		}, endpts); err != nil {
			return nil, err
		}
		return endpts, nil
	}
}

func EndpointsFuncForK8sClientset(
	cl *kubernetes.Clientset,
) GetEndpointsFunc {
	return func(
		ctx context.Context,
		namespace,
		serviceName string,
	) (*v1.Endpoints, error) {
		endpointsCl := cl.CoreV1().Endpoints(namespace)
		return endpointsCl.Get(ctx, serviceName, metav1.GetOptions{})
	}
}
