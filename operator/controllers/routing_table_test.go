package controllers

import (
	"context"
	"net/http"
	"strconv"
	"testing"

	"github.com/go-logr/logr"
	kedanet "github.com/kedacore/http-add-on/pkg/net"
	"github.com/kedacore/http-add-on/pkg/routing"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestRoutingTable(t *testing.T) {
	table := routing.NewTable()
	const (
		host     = "myhost.com"
		ns       = "testns"
		svcName  = "testsvc"
		deplName = "testdepl"
	)
	r := require.New(t)
	// create a new server (that we can introspect later on) to act
	// like a fake interceptor. we expect that pingInterceptors()
	// will make requests to this server
	hdl := kedanet.NewTestHTTPHandlerWrapper(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})
	srv, url, err := kedanet.StartTestServer(hdl)
	r.NoError(err)
	defer srv.Close()
	portInt, err := strconv.Atoi(url.Port())
	r.NoError(err)
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
	target := routing.Target{
		Service:    svcName,
		Port:       portInt,
		Deployment: deplName,
	}
	r.NoError(addAndUpdateRoutingTable(
		ctx,
		logr.Discard(),
		cl,
		table,
		host,
		target,
		ns,
		svcName,
		url.Port(),
	))

	reqs := hdl.IncomingRequests()
	r.Equal(len(endpoints.Subsets[0].Addresses), len(reqs))
	retTarget, err := table.Lookup(host)
	r.NoError(err)
	r.Equal(target, retTarget)

	r.NoError(removeAndUpdateRoutingTable(
		ctx,
		logr.Discard(),
		cl,
		table,
		host,
		ns,
		svcName,
		url.Port(),
	))
	reqs = hdl.IncomingRequests()
	r.Equal(len(endpoints.Subsets[0].Addresses)*2, len(reqs))
	retTarget, err = table.Lookup(host)
	r.Error(err)
}
