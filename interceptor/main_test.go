package main

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/kedacore/http-add-on/interceptor/config"
	kedanet "github.com/kedacore/http-add-on/pkg/net"
	"github.com/kedacore/http-add-on/pkg/queue"
	"github.com/kedacore/http-add-on/pkg/routing"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
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
	// see https://github.com/kedacore/http-add-on/issues/245
	// requires:
	// https://github.com/kedacore/http-add-on/pull/280
	// because that PR starts tests for the admin server.

}
