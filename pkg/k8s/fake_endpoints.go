package k8s

import (
	"net/url"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// FakeEndpointsForURL creates and returns a new *v1.Endpoints with a
// single v1.EndpointSubset in it, which has num v1.EndpointAddresses
// in it. Each of those EndpointAddresses has a Hostname and IP both
// equal to u.String()
func FakeEndpointsForURL(
	u *url.URL,
	namespace,
	name string,
	num int,
) *v1.Endpoints {
	addrs := make([]v1.EndpointAddress, num)
	for i := 0; i < num; i++ {
		addrs[i] = v1.EndpointAddress{
			Hostname: u.String(),
			IP:       u.String(),
		}
	}
	return &v1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Subsets: []v1.EndpointSubset{
			{
				Addresses: addrs,
			},
		},
	}
}
