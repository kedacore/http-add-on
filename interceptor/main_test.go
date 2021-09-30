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
	"github.com/kedacore/http-add-on/interceptor/config"
	"github.com/kedacore/http-add-on/pkg/k8s"
	"github.com/kedacore/http-add-on/pkg/queue"
	"github.com/kedacore/http-add-on/pkg/routing"
	"github.com/kedacore/http-add-on/pkg/test"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
)

func TestRunProxyServerCountMiddleware(t *testing.T) {
	// r := require.New(t)
	// ctx, done := context.WithCancel(
	// 	context.Background(),
	// )
	// defer done()
	// r.NoError(runProxyServer(ctx, logr.Discard(), q, waitFunc, routingTable, timeouts, port))

	// see https://github.com/kedacore/http-add-on/issues/245
}

func TestRunAdminServerDeploymentsEndpoint(t *testing.T) {
	// see https://github.com/kedacore/http-add-on/issues/245
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
