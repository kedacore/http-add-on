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
	cl := fake.NewClientBuilder().WithObjects(
		endpoints,
	).Build()
	urls, err := EndpointsForService(ctx, cl, ns, svcName, svcPort)
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
