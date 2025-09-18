package k8s

import (
	"fmt"
	"net/url"
	"strconv"

	discov1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

// FakeEndpointsForURL creates and returns a new *discov1.EndpointSlice with Endpoints
// in it. Each of those Endpoints has a Hostname and IP both
// equal to u.Hostname()
func FakeEndpointsForURL(u *url.URL, namespace, name string, num int) (*discov1.EndpointSliceList, error) {
	urls := make([]*url.URL, num)
	for i := 0; i < num; i++ {
		urls[i] = u
	}
	return FakeEndpointsForURLs(urls, namespace, name)
}

// FakeEndpointsForURLs creates and returns a new
// *discov1.EndpointSlice
// that has each url in the urls parameter in it.
func FakeEndpointsForURLs(urls []*url.URL, namespace, name string) (*discov1.EndpointSliceList, error) {
	es := &discov1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:         fmt.Sprintf("%s-%s", name, "96fhp"),
			GenerateName: name,
			Namespace:    namespace,
			Labels: map[string]string{
				discov1.LabelServiceName: name,
			},
		},
	}
	for _, u := range urls {
		es.Endpoints = append(es.Endpoints, discov1.Endpoint{
			Addresses: []string{
				u.Hostname(),
			},
			Hostname: ptr.To(u.Hostname()),
		})
		portInt, err := strconv.Atoi(u.Port())
		if err != nil {
			return nil, err
		}
		es.Ports = append(es.Ports, discov1.EndpointPort{
			Port: ptr.To(int32(portInt)),
		})
	}
	return &discov1.EndpointSliceList{Items: []discov1.EndpointSlice{*es}}, nil
}
