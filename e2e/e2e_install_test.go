package e2e

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/kedacore/http-add-on/pkg/k8s"
	"github.com/stretchr/testify/require"
)

// Install an HTTPScaledObject and ensure that the right things
// show up in the system
func TestE2EHTTPScaledObjectInstall(t *testing.T) {
	r := require.New(t)
	h, shouldRun, err := setup()
	if !shouldRun {
		t.Logf("Not running %s", t.Name())
		t.SkipNow()
	}
	r.NoError(err)
	defer h.close()
	ns := h.cfg.namespace()
	t.Logf("Running %s in namespace %s", t.Name(), ns)

	// need to use the lowercase so that it's a RFC 1123 compliant subdomain
	universalName := strings.ToLower(t.Name())
	scaledObject, err := k8s.NewScaledObject(
		ns,
		universalName,
		universalName,
		universalName,
		fmt.Sprintf("%s.com", universalName),
		0,
		100,
	)
	r.NoError(err)
	r.NoError(h.cl.Create(h, scaledObject))
	// make sure that the routing table is updated on the interceptor
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	r.NoError(tickMax(10, 100*time.Millisecond, func() error {
		return makeProxiedRequestsToSvc(
			h,
			h.restCfg,
			h.cfg.namespace(),
			h.cfg.ProxyAdminSvc,
			h.cfg.ProxyAdminPort,
			1,
		)
	}))
	r.NoError(tickMax(10, 100*time.Millisecond, func() error {
		return makeProxiedRequestsToSvc(
			h,
			h.restCfg,
			h.cfg.namespace(),
			h.cfg.OperatorAdminSvc,
			h.cfg.OperatorAdminPort,
			1,
		)
	}))
}
