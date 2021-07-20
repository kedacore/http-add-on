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
	lggr := logr.Discard()
	r := require.New(t)
	const port = 8080
	ctx, done := context.WithCancel(context.Background())
	defer done()
	errgrp, ctx := errgroup.WithContext(ctx)
	srvFunc := startHealthcheckServer(ctx, lggr, port)
	errgrp.Go(srvFunc)
	time.Sleep(500 * time.Millisecond)

	res, err := http.Get(fmt.Sprintf("http://0.0.0.0:%d/healthz", port))
	r.NoError(err)
	r.Equal(200, res.StatusCode)

	res, err = http.Get(fmt.Sprintf("http://0.0.0.0:%d/livez", port))
	r.NoError(err)
	r.Equal(200, res.StatusCode)
}
