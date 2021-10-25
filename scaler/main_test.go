package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
	"k8s.io/apimachinery/pkg/util/rand"
)

func TestHealthChecks(t *testing.T) {
	ctx := context.Background()
	ctx, done := context.WithCancel(ctx)
	defer done()
	lggr := logr.Discard()
	r := require.New(t)
	port := rand.Intn(100) + 8000
	cfg := &config{
		GRPCPort:              port + 1,
		HealthPort:            port,
		TargetNamespace:       "test123",
		TargetService:         "testsvc",
		TargetPort:            port + 123,
		TargetPendingRequests: 100,
		UpdateRoutingTableDur: 100 * time.Millisecond,
	}

	errgrp, ctx := errgroup.WithContext(ctx)

	ticker, pinger, err := newFakeQueuePinger(ctx, lggr)
	r.NoError(err)
	defer ticker.Stop()
	srvFunc := func() error {
		return startHealthcheckServer(
			ctx,
			lggr,
			cfg,
			port,
			pinger,
		)
	}

	newURL := func(path string) string {
		return fmt.Sprintf("http://0.0.0.0:%d/%s", port, path)
	}
	errgrp.Go(srvFunc)
	time.Sleep(500 * time.Millisecond)

	res, err := http.Get(newURL("healthz"))
	r.NoError(err)
	defer res.Body.Close()
	r.Equal(200, res.StatusCode)

	res, err = http.Get(newURL("livez"))
	r.NoError(err)
	defer res.Body.Close()
	r.Equal(200, res.StatusCode)

	res, err = http.Get(newURL("config"))
	r.NoError(err)
	defer res.Body.Close()
	r.Equal(200, res.StatusCode)
	bodyBytes, err := io.ReadAll(res.Body)
	r.NoError(err)
	retCfg := map[string][]config{}
	r.NoError(json.Unmarshal(bodyBytes, &retCfg))
	expected := map[string][]config{
		"configs": {*cfg},
	}
	r.Equal(expected, retCfg)

	done()
	r.Error(errgrp.Wait())
}
