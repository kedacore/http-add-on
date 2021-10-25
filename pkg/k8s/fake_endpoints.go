package k8s

import (
	"net/url"
	"strconv"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// FakeEndpointsForURL creates and returns a new *v1.Endpoints with a
// single v1.EndpointSubset in it, which has num v1.EndpointAddresses
// in it. Each of those EndpointAddresses has a Hostname and IP both
// equal to u.Hostname()
func FakeEndpointsForURL(
	u *url.URL,
	namespace,
	name string,
	num int,
) (*v1.Endpoints, error) {
	urls := make([]*url.URL, num)
	for i := 0; i < num; i++ {
		urls[i] = u
	}
	return FakeEndpointsForURLs(urls, namespace, name)
}

// FakeEndpointsForURLs creates and returns a new
// *v1.Endpoints with a single v1.EndpointSubset in it
// that has each url in the urls parameter in it.
func FakeEndpointsForURLs(
	urls []*url.URL,
	namespace,
	name string,
) (*v1.Endpoints, error) {
	addrs := make([]v1.EndpointAddress, len(urls))
	ports := make([]v1.EndpointPort, len(urls))
	for i, u := range urls {
		addrs[i] = v1.EndpointAddress{
			Hostname: u.Hostname(),
			IP:       u.Hostname(),
		}
		portInt, err := strconv.Atoi(u.Port())
		if err != nil {
			return nil, err
		}
		ports[i] = v1.EndpointPort{
			Port: int32(portInt),
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
				Ports:     ports,
			},
		},
	}, nil
}
