package e2e

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/kedacore/http-add-on/pkg/k8s"
	"github.com/stretchr/testify/require"
)

// Install an HTTPScaledObject and ensure that the right things
// show up in the system
func TestE2EHTTPScaledObjectInstall(t *testing.T) {
	r := require.New(t)
	cfg, shouldRun, err := parseConfig()
	if !shouldRun {
		t.Logf("Not running %s", t.Name())
		t.SkipNow()
	}
	r.NoError(err)
	ns := cfg.namespace()
	t.Logf("Running %s in namespace %s", t.Name(), ns)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() { cancel() })

	// install an HTTPScaledObject
	cl, restCfg, err := getClient()
	r.NoError(err)
	scaledObject, err := k8s.NewScaledObject(
		ns,
		t.Name(),
		t.Name(),
		t.Name(),
		fmt.Sprintf("%s.com", t.Name()),
		0,
		100,
	)
	r.NoError(err)
	r.NoError(cl.Create(ctx, scaledObject))
	// make sure that the routing table is updated on the interceptor
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	r.NoError(tickMax(10, 100*time.Millisecond, func() error {
		return makeProxiedRequestsToSvc(
			ctx,
			restCfg,
			cfg.namespace(),
			cfg.ProxyAdminSvc,
			cfg.ProxyAdminPort,
			1,
		)
	}))
	r.NoError(tickMax(10, 100*time.Millisecond, func() error {
		return makeProxiedRequestsToSvc(
			ctx,
			restCfg,
			cfg.namespace(),
			cfg.OperatorAdminSvc,
			cfg.OperatorAdminPort,
			1,
		)
	}))
}

func tickMax(
	maxTries int,
	tickDur time.Duration,
	f func() error,
) error {
	var lastErr error
	ticker := time.NewTicker(tickDur)
	defer ticker.Stop()
	for i := 0; i < maxTries; i++ {
		<-ticker.C
		err := f()
		if err == nil {
			lastErr = nil
			break
		} else {
			lastErr = err
		}
	}
	return lastErr
}
