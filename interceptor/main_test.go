package main

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/kedacore/http-add-on/interceptor/config"
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
	g, ctx := errgroup.WithContext(ctx)
	q := queue.NewFakeCounter()
	routingTable := routing.NewTable()
	routingTable.AddTarget(
		host,
		routing.NewTarget(
			"samplesvc",
			123,
			"sampledepl",
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
		resp, err := http.Get(fmt.Sprintf(
			"http://0.0.0.0:%d", port,
		))
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
		r.Equal(1, hostAndCount.Count)
	case <-time.After(500 * time.Millisecond):
		r.Fail("timeout waiting for queue resize")
	}

	done()
	r.Error(g.Wait())
}

func TestRunAdminServerDeploymentsEndpoint(t *testing.T) {
	// see https://github.com/kedacore/http-add-on/issues/245
}
