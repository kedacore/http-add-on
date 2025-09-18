package k8s

import (
	"context"
	"fmt"
	"maps"
	"net/url"
	"slices"

	discov1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Endpoints represents a set of ready and not ready addresses
type Endpoints struct {
	ReadyAddresses    []string
	NotReadyAddresses []string
}

// GetEndpointsFunc is a type that represents a function that can
// fetch endpoints
type GetEndpointsFunc func(ctx context.Context, namespace, serviceName string) (Endpoints, error)

// EndpointsForService fetches the ready endpoints for a given service and returns them as a slice of URLs with the specified port
func EndpointsForService(ctx context.Context, ns, serviceName, servicePort string, endpointsFn GetEndpointsFunc) ([]url.URL, error) {
	endpoints, err := endpointsFn(ctx, ns, serviceName)
	if err != nil {
		return []url.URL{}, fmt.Errorf("pkg.k8s.EndpointsForService: %w", err)
	}
	ret := make([]url.URL, 0, len(endpoints.ReadyAddresses))
	for _, addr := range endpoints.ReadyAddresses {
		u := url.URL{
			Scheme: "http",
			Host:   fmt.Sprintf("%s:%s", addr, servicePort),
		}
		ret = append(ret, u)
	}
	return ret, nil
}

// EndpointsFuncForControllerClient returns a new GetEndpointsFunc that uses the controller-runtime client.Client to fetch endpoints
func EndpointsFuncForControllerClient(cl client.Client) GetEndpointsFunc {
	return func(ctx context.Context, namespace, serviceName string) (Endpoints, error) {
		ess := &discov1.EndpointSliceList{}

		if err := cl.List(ctx, ess, client.InNamespace(namespace), client.MatchingLabels{discov1.LabelServiceName: serviceName}); err != nil {
			return Endpoints{}, err
		}
		return extractAddresses(ess.Items), nil
	}
}

// EndpointsFuncForK8sClientset returns a new GetEndpointsFunc that uses the kubernetes.Clientset to fetch endpoints
// TODO: this should be eventually removed because it causes high load on the API server, there is EndpointsFuncForControllerClient instead
func EndpointsFuncForK8sClientset(cl *kubernetes.Clientset) GetEndpointsFunc {
	return func(ctx context.Context, namespace, serviceName string) (Endpoints, error) {
		endpointSlCl := cl.DiscoveryV1().EndpointSlices(namespace)
		ess, err := endpointSlCl.List(ctx, metav1.ListOptions{LabelSelector: fmt.Sprintf("%s=%s", discov1.LabelServiceName, serviceName)})
		if err != nil {
			return Endpoints{}, err
		}
		return extractAddresses(ess.Items), nil
	}
}

// extractAddresses extracts ready and not ready addresses from the given list of EndpointSlice
func extractAddresses(eps []discov1.EndpointSlice) Endpoints {
	// addresses from EndpointSlices need to be deduplicated
	ready, notReady := make(map[string]bool), make(map[string]bool)
	for _, ep := range eps {
		for _, addr := range ep.Endpoints {
			// see also https://github.com/kubernetes/api/blob/v0.33.0/discovery/v1/types.go#L137-L144
			if addr.Conditions.Ready == nil || *addr.Conditions.Ready {
				for _, a := range addr.Addresses {
					ready[a] = true
				}
			} else {
				for _, a := range addr.Addresses {
					notReady[a] = true
				}
			}
		}
	}
	return Endpoints{
		ReadyAddresses:    slices.Collect(maps.Keys(ready)),
		NotReadyAddresses: slices.Collect(maps.Keys(notReady)),
	}
}
