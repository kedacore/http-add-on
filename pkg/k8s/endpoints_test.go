package k8s

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	discov1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestGetEndpoints(t *testing.T) {
	r := require.New(t)
	ctx := context.Background()

	const (
		ns      = "testns"
		svcName = "testsvc"
		svcPort = "8081"
	)
	endpoints := &discov1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:         fmt.Sprintf("%s-%s", svcName, "96fhp"),
			GenerateName: svcName,
			Namespace:    ns,
			Labels: map[string]string{
				discov1.LabelServiceName: svcName,
			},
		},
		Endpoints: []discov1.Endpoint{
			{
				Addresses: []string{
					"1.2.3.4",
				},
				Hostname: ptr.To("testhost1"),
			},
			{
				Addresses: []string{
					"1.2.3.5",
				},
				Hostname: ptr.To("testhost2"),
			},
		},
	}
	urls, err := EndpointsForService(
		ctx,
		ns,
		svcName,
		svcPort,
		func(context.Context, string, string) (Endpoints, error) {
			return extractAddresses([]discov1.EndpointSlice{*endpoints}), nil
		},
	)
	r.NoError(err)
	addrLookup := map[string]*string{}
	for _, es := range endpoints.Endpoints {
		for _, addr := range es.Addresses {
			key := fmt.Sprintf("http://%s:%s", addr, svcPort)
			addrLookup[key] = &addr
		}
	}
	r.Len(urls, len(addrLookup))
	for _, url := range urls {
		_, ok := addrLookup[url.String()]
		r.True(ok, "address %s was returned but not expected", url)
	}
}

func TestEndpointsFuncForControllerClient(t *testing.T) {
	ctx := context.Background()
	const (
		ns      = "testns"
		svcName = "testsvc"
		svcPort = 8081
	)
	r := require.New(t)
	endpoints := &discov1.EndpointSliceList{
		Items: []discov1.EndpointSlice{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:         fmt.Sprintf("%s-%s", svcName, "96fhp"),
					GenerateName: svcName,
					Namespace:    ns,
					Labels: map[string]string{
						discov1.LabelServiceName: svcName,
					},
				},
				Ports: []discov1.EndpointPort{
					{
						Port: ptr.To(int32(svcPort)),
					},
				},
				Endpoints: []discov1.Endpoint{
					{
						Addresses: []string{
							"1.2.3.4",
						},
						Hostname: ptr.To("testhost1"),
					},
					{
						Addresses: []string{
							"2.3.4.5",
						},
						Hostname: ptr.To("testhost2"),
					},
				},
			},
		},
	}

	cl := fake.NewClientBuilder().WithLists(endpoints).Build()
	fn := EndpointsFuncForControllerClient(cl)
	ret, err := fn(ctx, ns, svcName)
	r.NoError(err)
	r.Len(ret.ReadyAddresses, len(endpoints.Items[0].Endpoints))
	// we don't need to introspect the return value, because we
	// do so in depth in the above TestGetEndpoints test
}
