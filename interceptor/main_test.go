package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/kedacore/http-add-on/interceptor/config"
	"github.com/kedacore/http-add-on/pkg/k8s"
	kedanet "github.com/kedacore/http-add-on/pkg/net"
	"github.com/kedacore/http-add-on/pkg/queue"
	"github.com/kedacore/http-add-on/pkg/routing"
	"github.com/kedacore/http-add-on/pkg/test"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
)

func TestRunProxyServerCountMiddleware(t *testing.T) {
	const (
		port = 8080
		host = "samplehost"
	)
	r := require.New(t)
	ctx, done := context.WithCancel(
		context.Background(),
	)
	defer done()

	originHdl := kedanet.NewTestHTTPHandlerWrapper(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)
	originSrv, originURL, err := kedanet.StartTestServer(originHdl)
	r.NoError(err)
	defer originSrv.Close()
	originPort, err := strconv.Atoi(originURL.Port())
	r.NoError(err)
	g, ctx := errgroup.WithContext(ctx)
	q := queue.NewFakeCounter()
	routingTable := routing.NewTable()
	// set up a fake host that we can spoof
	// when we later send request to the proxy,
	// so that the proxy calculates a URL for that
	// host that points to the (above) fake origin
	// server.
	routingTable.AddTarget(
		host,
		targetFromURL(
			originURL,
			originPort,
			"testdepl",
			123,
		),
	)
	timeouts := &config.Timeouts{}
	waiterCh := make(chan struct{})
	waitFunc := func(ctx context.Context, name string) error {
		<-waiterCh
		return nil
	}
	g.Go(func() error {
		return runProxyServer(
			ctx,
			logr.Discard(),
			q,
			waitFunc,
			routingTable,
			timeouts,
			port,
		)
	})
	// wait for server to start
	time.Sleep(500 * time.Millisecond)

	// make an HTTP request in the background
	g.Go(func() error {
		req, err := http.NewRequest(
			"GET",
			fmt.Sprintf(
				"http://0.0.0.0:%d", port,
			), nil,
		)
		if err != nil {
			return err
		}
		req.Host = host
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf(
				"unexpected status code: %d",
				resp.StatusCode,
			)
		}
		return nil
	})
	time.Sleep(100 * time.Millisecond)
	select {
	case hostAndCount := <-q.ResizedCh:
		r.Equal(host, hostAndCount.Host)
		r.Equal(+1, hostAndCount.Count)
	case <-time.After(500 * time.Millisecond):
		r.Fail("timeout waiting for +1 queue resize")
	}

	// tell the wait func to proceed
	waiterCh <- struct{}{}

	select {
	case hostAndCount := <-q.ResizedCh:
		r.Equal(host, hostAndCount.Host)
		r.Equal(-1, hostAndCount.Count)
	case <-time.After(500 * time.Millisecond):
		r.Fail("timeout waiting for -1 queue resize")
	}

	// check the queue to make sure all counts are at 0
	countsPtr, err := q.Current()
	r.NoError(err)
	counts := countsPtr.Counts
	r.Equal(1, len(counts))
	_, foundHost := counts[host]
	r.True(
		foundHost,
		"couldn't find host %s in the queue",
		host,
	)
	r.Equal(0, counts[host])

	done()
	r.Error(g.Wait())
}

func TestRunAdminServerDeploymentsEndpoint(t *testing.T) {

	ctx := context.Background()
	ctx, done := context.WithCancel(ctx)
	defer done()
	lggr := logr.Discard()
	r := require.New(t)
	port := rand.Intn(100) + 8000
	const deplName = "testdeployment"
	srvCfg := &config.Serving{}
	timeoutCfg := &config.Timeouts{}

	deplCache := k8s.NewFakeDeploymentCache()
	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		return runAdminServer(
			ctx,
			lggr,
			k8s.FakeConfigMapGetter{},
			queue.NewFakeCounter(),
			routing.NewTable(),
			deplCache,
			port,
			srvCfg,
			timeoutCfg,
		)
	})
	time.Sleep(500 * time.Millisecond)

	deplCache.Set(
		deplName,
		appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name: deplName,
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: k8s.Int32P(123),
			},
		},
	)

	res, err := http.Get(fmt.Sprintf("http://0.0.0.0:%d/deployments", port))
	r.NoError(err)
	defer res.Body.Close()
	r.Equal(200, res.StatusCode)

	actual := map[string]int32{}
	r.NoError(json.NewDecoder(res.Body).Decode(&actual))

	expected := map[string]int32{}
	for name, depl := range deplCache.Current {
		expected[name] = *depl.Spec.Replicas
	}

	r.Equal(expected, actual)

	done()
	r.Error(g.Wait())
}

func TestRunAdminServerConfig(t *testing.T) {
	ctx := context.Background()
	ctx, done := context.WithCancel(ctx)
	defer done()
	lggr := logr.Discard()
	r := require.New(t)
	const port = 8080
	srvCfg := &config.Serving{}
	timeoutCfg := &config.Timeouts{}

	errgrp, ctx := errgroup.WithContext(ctx)

	errgrp.Go(func() error {
		return runAdminServer(
			ctx,
			lggr,
			k8s.FakeConfigMapGetter{},
			queue.NewFakeCounter(),
			routing.NewTable(),
			k8s.NewFakeDeploymentCache(),
			port,
			srvCfg,
			timeoutCfg,
		)
	})
	time.Sleep(500 * time.Millisecond)

	urlStr := func(path string) string {
		return fmt.Sprintf("http://0.0.0.0:%d/%s", port, path)
	}
	res, err := http.Get(urlStr("config"))
	r.NoError(err)
	defer res.Body.Close()
	r.Equal(200, res.StatusCode)

	bodyBytes, err := io.ReadAll(res.Body)
	r.NoError(err)

	decodedIfaces := map[string][]interface{}{}
	r.NoError(json.Unmarshal(bodyBytes, &decodedIfaces))
	r.Equal(1, len(decodedIfaces))
	_, hasKey := decodedIfaces["configs"]
	r.True(hasKey, "config body doesn't have 'configs' key")
	configs := decodedIfaces["configs"]
	r.Equal(2, len(configs))

	retSrvCfg := &config.Serving{}
	r.NoError(test.JSONRoundTrip(configs[0], retSrvCfg))
	retTimeoutsCfg := &config.Timeouts{}
	r.NoError(test.JSONRoundTrip(configs[1], retTimeoutsCfg))
	r.Equal(*srvCfg, *retSrvCfg)
	r.Equal(*timeoutCfg, *retTimeoutsCfg)

	done()
	r.Error(errgrp.Wait())
}
