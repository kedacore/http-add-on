package k8s

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	endpoints := &v1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:      svcName,
			Namespace: ns,
		},
		Subsets: []v1.EndpointSubset{
			{
				Addresses: []v1.EndpointAddress{
					{
						IP:       "1.2.3.4",
						Hostname: "testhost1",
					},
				},
			},
			{
				Addresses: []v1.EndpointAddress{
					{
						IP:       "2.3.4.5",
						Hostname: "testhost2",
					},
				},
			},
		},
	}
	urls, err := EndpointsForService(
		ctx,
		ns,
		svcName,
		svcPort,
		func(context.Context, string, string) (*v1.Endpoints, error) {
			return endpoints, nil
		},
	)
	r.NoError(err)
	addrLookup := map[string]*v1.EndpointAddress{}
	for _, subset := range endpoints.Subsets {
		for _, addr := range subset.Addresses {
			key := fmt.Sprintf("http://%s:%s", addr.IP, svcPort)
			addrLookup[key] = &addr
		}
	}
	r.Equal(len(addrLookup), len(urls))
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
		svcPort = "8081"
	)
	r := require.New(t)
	endpoints := &v1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:      svcName,
			Namespace: ns,
		},
		Subsets: []v1.EndpointSubset{
			{
				Addresses: []v1.EndpointAddress{
					{
						IP:       "1.2.3.4",
						Hostname: "testhost1",
					},
				},
			},
			{
				Addresses: []v1.EndpointAddress{
					{
						IP:       "2.3.4.5",
						Hostname: "testhost2",
					},
				},
			},
		},
	}
	cl := fake.NewClientBuilder().WithObjects(
		endpoints,
	).Build()
	fn := EndpointsFuncForControllerClient(cl)
	ret, err := fn(ctx, ns, svcName)
	r.NoError(err)
	r.Equal(len(endpoints.Subsets), len(ret.Subsets))
	// we don't need to introspect the return value, because we
	// do so in depth in the above TestGetEndpoints test
}
