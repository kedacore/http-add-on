package main

import (
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/kedacore/http-add-on/pkg/k8s"
	kedanet "github.com/kedacore/http-add-on/pkg/net"
	"github.com/kedacore/http-add-on/pkg/routing"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

func TestIntegration(t *testing.T) {
	const (
		deploymentReplicasTimeout = 200 * time.Millisecond
		responseHeaderTimeout     = 1 * time.Second
		host                      = "integrationtest.interceptor.kedattp.dev"
		deplName                  = "testdeployment"
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
	numReplicas := int32(3)
	deplCache.Set(deplName, &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: deplName},
		Spec: appsv1.DeploymentSpec{
			Replicas: &numReplicas,
		},
	})
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
	t.Logf("Origin server started on %s", originSrvURL.String())
	originPortInt, err := strconv.Atoi(originSrvURL.Port())
	r.NoError(err)
	routingTable.AddTarget(host, routing.Target{
		// url.Host can contain the port, so ensure we only get the actual host
		Service:    strings.Split(originSrvURL.Host, ":")[0],
		Port:       originPortInt,
		Deployment: deplName,
	})

	proxySrv, proxySrvURL, err := kedanet.StartTestServer(proxyHdl)
	r.NoError(err)
	defer proxySrv.Close()
	t.Logf("Proxy server started on %s", proxySrvURL.String())

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
