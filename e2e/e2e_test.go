package e2e

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/kelseyhightower/envconfig"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/util/uuid"
)

func TestE2E(t *testing.T) {
	shouldRun := os.Getenv("KEDA_HTTP_E2E_SHOULD_RUN")
	if shouldRun != "true" {
		t.Logf("Not running E2E Tests")
		t.SkipNow()
	}

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

	// issue requests to the XKCD service directly to make
	// sure it's up and properly configured
	r.NoError(makeRequestsToSvc(
		ctx,
		restCfg,
		ns,
		"xkcd",
		8080,
		cfg.NumReqsAgainstProxy,
	))

	// issue requests to the proxy service to make sure
	// it's forwarding properly
	r.NoError(makeRequestsToSvc(
		ctx,
		restCfg,
		ns,
		"keda-add-ons-http-interceptor-proxy",
		8080,
		cfg.NumReqsAgainstProxy,
	))
}
