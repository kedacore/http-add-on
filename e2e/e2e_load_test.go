package e2e

import (
	"context"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestE2ELoad(t *testing.T) {
	cfg, shouldRun, err := parseConfig()
	if !shouldRun {
		t.Logf("Not running %s", t.Name())
		t.SkipNow()
	}

	ctx, cancel := context.WithCancel(context.Background())
	r := require.New(t)
	ns := cfg.namespace()
	t.Logf("Running %s in namespace %s", t.Name(), ns)

	// setup and register teardown functionality.
	// register cleanup before executing setup, so that
	// if setup times out, we'll still clean up
	t.Cleanup(func() { cancel() })

	cl, restCfg, err := getClient()
	r.NoError(err)

	// ensure that the interceptor and XKCD scaledobjects
	// exist
	_, err = getScaledObject(ctx, cl, ns, "keda-add-ons-http-interceptor")
	r.NoError(err)
	_, err = getScaledObject(ctx, cl, ns, "xkcd-app")
	r.NoError(err)

	// issue requests to the XKCD service directly to make
	// sure it's up and properly configured
	ingURL, err := url.Parse(cfg.IngAddress)
	r.NoError(err)
	r.NoError(makeRequests(
		ctx,
		http.DefaultClient,
		ingURL,
		cfg.NumReqsAgainstProxy,
		func() error {
			if err := checkInterceptorMetrics(
				ctx,
				restCfg,
				ns,
				cfg.ProxyAdminSvc,
				cfg.ProxyAdminPort,
			); err != nil {
				return err
			}

			if err := checkScalerMetrics(
				ctx,
				restCfg,
				ns,
				cfg.ScalerAdminSvc,
				cfg.ScalerAdminPort,
			); err != nil {
				return err
			}
			return nil
		},
		cfg.AdminServerCheckDur,
	))
}
