package controllers

import (
	"context"
	"net/http"
	"testing"

	kedanet "github.com/kedacore/http-add-on/pkg/net"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
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
	endpoints := &v1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:      svcName,
			Namespace: ns,
		},
		Subsets: []v1.EndpointSubset{
			{
				Addresses: []v1.EndpointAddress{
					{
						Hostname: url.String(),
					},
					{
						Hostname: url.String(),
					},
				},
			},
		},
	}
	cl := fake.NewClientBuilder().WithObjects(endpoints).Build()
	r.NoError(pingInterceptors(
		ctx,
		cl,
		srv.Client(),
		ns,
		svcName,
		url.Port(),
	))
	reqs := hdl.IncomingRequests()
	r.Equal(len(endpoints.Subsets[0].Addresses), len(reqs))
}
