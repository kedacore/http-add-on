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
	"github.com/kedacore/http-add-on/operator/controllers/config"
	"github.com/kedacore/http-add-on/pkg/routing"
	"github.com/kedacore/http-add-on/pkg/test"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
	"k8s.io/apimachinery/pkg/util/rand"
)

func TestRunAdminServerConfig(t *testing.T) {
	ctx := context.Background()
	ctx, done := context.WithCancel(ctx)
	defer done()
	lggr := logr.Discard()
	r := require.New(t)
	port := rand.Intn(100) + 8000
	baseCfg := &config.Base{}
	interceptorCfg := &config.Interceptor{}
	externalScalerCfg := &config.ExternalScaler{}

	errgrp, ctx := errgroup.WithContext(ctx)

	errgrp.Go(func() error {
		return runAdminServer(
			ctx,
			lggr,
			routing.NewTable(),
			port,
			baseCfg,
			interceptorCfg,
			externalScalerCfg,
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
	r.Equal(3, len(configs))

	retBaseCfg := &config.Base{}
	r.NoError(test.JSONRoundTrip(configs[0], retBaseCfg))
	retInterceptorCfg := &config.Interceptor{}
	r.NoError(test.JSONRoundTrip(configs[1], retInterceptorCfg))
	retExternalScalerCfg := &config.ExternalScaler{}
	r.NoError(test.JSONRoundTrip(configs[2], retExternalScalerCfg))
	r.Equal(*baseCfg, *retBaseCfg)
	r.Equal(*interceptorCfg, *retInterceptorCfg)
	r.Equal(*externalScalerCfg, *retExternalScalerCfg)

	done()
	r.Error(errgrp.Wait())

}
