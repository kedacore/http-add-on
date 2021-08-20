package main

import (
	"io"
	"net"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/kedacore/http-add-on/pkg/k8s"
	kedanet "github.com/kedacore/http-add-on/pkg/net"
	"github.com/kedacore/http-add-on/pkg/routing"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/util/wait"
)

func TestIntegration(t *testing.T) {
	const (
		deploymentReplicasTimeout = 1 * time.Second
		responseHeaderTimeout     = 1 * time.Second
		host                      = "integrationtest.interceptor.kedattp.dev"
	)
	r := require.New(t)
	lggr := logr.Discard()
	routingTable := routing.NewTable()
	dialContextFunc := kedanet.DialContextWithRetry(
		&net.Dialer{
			Timeout: 2 * time.Second,
		},
		wait.Backoff{
			Steps:    2,
			Duration: time.Second,
		},
	)

	deplCache := k8s.NewFakeDeploymentCache()
	waitFunc := newDeployReplicasForwardWaitFunc(deplCache)

	proxyHdl := newForwardingHandler(
		lggr,
		routingTable,
		dialContextFunc,
		waitFunc,
		deploymentReplicasTimeout,
		responseHeaderTimeout,
	)

	testOriginSrv, originSrvURL, err := kedanet.StartTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("hello!"))
	}))
	r.NoError(err)
	defer testOriginSrv.Close()
	originPortInt, err := strconv.Atoi(originSrvURL.Port())
	r.NoError(err)
	routingTable.AddTarget(host, routing.Target{
		Service: originSrvURL.Host,
		Port:    originPortInt,
	})

	proxySrv, proxySrvURL, err := kedanet.StartTestServer(proxyHdl)
	r.NoError(err)
	defer proxySrv.Close()

	// happy path
	res, err := doRequest(
		http.DefaultClient,
		"GET",
		proxySrvURL.String(),
		host,
		nil,
	)
	r.NoError(err)
	r.Equal(200, res.StatusCode)

	// not in the routing table
	res, err = doRequest(
		http.DefaultClient,
		"GET",
		proxySrvURL.String(),
		"not-in-the-table",
		nil,
	)
	r.NoError(err)
	r.Equal(404, res.StatusCode)

	// 0 replicas
	deplCache.SetReplicas(host, 0)
	res, err = doRequest(
		http.DefaultClient,
		"GET",
		proxySrvURL.String(),
		host,
		nil,
	)
	r.NoError(err)
	r.Equal(502, res.StatusCode)
}

func doRequest(
	cl *http.Client,
	method,
	urlStr,
	host string,
	body io.ReadCloser,
) (*http.Response, error) {
	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		return nil, err
	}
	req.Host = host
	res, err := cl.Do(req)
	if err != nil {
		return nil, err
	}
	return res, nil
}
