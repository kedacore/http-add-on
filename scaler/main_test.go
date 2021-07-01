package main

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
)

func TestHealthChecks(t *testing.T) {
	ctx := context.Background()
	ctx, done := context.WithCancel(ctx)
	defer done()
	lggr := logr.Discard()
	r := require.New(t)
	const port = 8080

	errgrp, ctx := errgroup.WithContext(ctx)

	ticker, pinger := newFakeQueuePinger(ctx, lggr)
	defer ticker.Stop()
	srvFunc := startHealthcheckServer(ctx, lggr, port, pinger)
	errgrp.Go(srvFunc)
	time.Sleep(500 * time.Millisecond)

	res, err := http.Get(fmt.Sprintf("http://0.0.0.0:%d/healthz", port))
	r.NoError(err)
	r.Equal(200, res.StatusCode)

	res, err = http.Get(fmt.Sprintf("http://0.0.0.0:%d/livez", port))
	r.NoError(err)
	r.Equal(200, res.StatusCode)
}
