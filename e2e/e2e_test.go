package e2e

import (
	"context"
	"fmt"
	"testing"

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

	// setup and register teardown functionality
	setup(t, ns, cfg)
	t.Cleanup(func() {
		cancel()
		teardown(t, ns)
	})

	// get the address of the interceptor proxy service and ping it.
	// it should make a request all the way back to the example app
	cl, restCfg, err := getClient()
	r.NoError(err)
	proxySvcName := "keda-add-ons-http-interceptor-proxy"
	proxySvc := &corev1.Service{}
	r.NoError(cl.Get(ctx, objKey(ns, proxySvcName), proxySvc))
	pf, err := tunnelSvc(t, ctx, restCfg, proxySvc)
	r.NoError(err)
	defer pf.Close()
	r.NoError(pf.ForwardPorts())

}
