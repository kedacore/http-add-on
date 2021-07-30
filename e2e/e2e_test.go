package e2e

import (
	"context"
	"fmt"
	"testing"

	"github.com/kedacore/http-add-on/pkg/k8s"
	"github.com/kelseyhightower/envconfig"
	"github.com/stretchr/testify/require"
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

	cl, _ /*restCfg*/, err := getClient()
	r.NoError(err)

	// ensure that the HTTPScaledObject has a proper status,
	// and that a ScaledObject was created for the example app
	scaledObjectName := fmt.Sprintf("%s-app", "xkcd")
	scaledObject, err := k8s.NewScaledObject(ns, scaledObjectName, "", "", "", 1, 2)
	r.NoError(err)
	r.NoError(cl.Get(ctx, objKey(ns, scaledObjectName), scaledObject))

	// get the address of the interceptor proxy service and ping it.
	// it should make a request all the way back to the example app
	// proxySvcName := "keda-add-ons-http-interceptor-proxy"
	// proxySvc := &corev1.Service{}
	// r.NoError(cl.Get(ctx, objKey(ns, proxySvcName), proxySvc))
	// pf, err := tunnelSvc(t, ctx, restCfg, proxySvc)
	// r.NoError(err)
	// defer pf.Close()
	// r.NoError(pf.ForwardPorts())

}
