package http

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/kedacore/http-add-on/pkg/k8s"
	kedanet "github.com/kedacore/http-add-on/pkg/net"
)

func TestPingInterceptors(t *testing.T) {
	const (
		ns      = "testns"
		svcName = "testsvc"
	)
	r := require.New(t)
	// create a new server (that we can introspect later on) to act
	// like a fake interceptor. we expect that pingInterceptors()
	// will make requests to this server
	hdl := kedanet.NewTestHTTPHandlerWrapper(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(200)
		}),
	)
	srv, url, err := kedanet.StartTestServer(hdl)
	r.NoError(err)
	defer srv.Close()
	ctx := context.Background()
	endpoints, err := k8s.FakeEndpointsForURL(url, ns, svcName, 1)
	r.NoError(err)
	eps := []client.Object{}
	for i := range endpoints.Items {
		eps = append(eps, &endpoints.Items[i])
	}
	cl := fake.NewClientBuilder().WithObjects(eps...).Build()
	r.NoError(pingInterceptors(
		ctx,
		cl,
		srv.Client(),
		ns,
		svcName,
		url.Port(),
	))
	reqs := hdl.IncomingRequests()
	var endpointsAddrs []string
	for _, es := range endpoints.Items {
		for _, e := range es.Endpoints {
			endpointsAddrs = append(endpointsAddrs, e.Addresses...)
		}
	}
	r.Equal(len(endpointsAddrs), len(reqs))
}
