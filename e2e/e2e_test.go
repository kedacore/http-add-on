package e2e

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/kelseyhightower/envconfig"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
)

func TestE2E(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	r := require.New(t)

	ns := fmt.Sprintf("keda-http-add-on-e2e-%s", uuid.NewUUID())
	cfg := new(config)
	envconfig.MustProcess("KEDA_HTTP_E2E", cfg)

	if cfg.Namespace != "" {
		ns = cfg.Namespace
	}

	t.Logf("E2E Tests Starting")
	t.Logf("Using namespace: %s", ns)

	// setup and register teardown functionality.
	// register cleanup before executing setup, so that
	// if setup times out, we'll still clean up
	t.Cleanup(func() {
		cancel()
		if cfg.RunSetupTeardown {
			teardown(t, ns)
		}
	})
	if cfg.RunSetupTeardown {
		t.Logf("Running setup and teardown scripts")
		setup(t, ns, cfg)
	}

	cl, restCfg, err := getClient()
	r.NoError(err)

	// wait until all expected deployments are available
	r.NoError(waitUntilDeplomentsAvailable(
		ctx,
		cl,
		remainingDurInTest(t, 20*time.Second),
		ns,
		[]string{
			"keda-operator",
			"keda-add-ons-http-controller-manager",
			"keda-add-ons-http-external-scaler",
			"keda-add-ons-http-interceptor",
			"keda-operator-metrics-apiserver",
			"xkcd",
		},
	))

	// ensure that the interceptor and XKCD scaledobjects
	// exist
	_, err = getScaledObject(ctx, cl, ns, "keda-add-ons-http-interceptor")
	r.NoError(err)
	_, err = getScaledObject(ctx, cl, ns, "xkcd-app")
	r.NoError(err)

	// get the address of the interceptor proxy service and ping it.
	// it should make a request all the way back to the example app
	proxySvcName := "keda-add-ons-http-interceptor-proxy"
	proxySvc := &corev1.Service{}
	r.NoError(cl.Get(ctx, objKey(ns, proxySvcName), proxySvc))
	pf, err := tunnelSvc(t, ctx, restCfg, proxySvc)
	r.NoError(err)
	defer pf.Close()
	r.NoError(pf.ForwardPorts())

}
