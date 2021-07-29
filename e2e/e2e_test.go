package main

import (
	"fmt"
	"testing"

	"github.com/kelseyhightower/envconfig"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/util/uuid"
)

func TestE2E(t *testing.T) {
	r := require.New(t)

	ns := fmt.Sprintf("keda-http-add-on-e2e-%s", uuid.NewUUID())
	cfg := new(config)
	envconfig.MustProcess("KEDA_HTTP_E2E", cfg)

	if cfg.Namespace != "" {
		ns = cfg.Namespace
	}

	t.Logf("E2E Tests Starting")
	t.Logf("Using namespace: %s", ns)

	r.NoError(helmRepoAdd("kedacore", "https://kedacore.github.io/charts"))
	r.NoError(helmRepoUpdate())
	r.NoError(helmInstall(ns, "keda", "kedacore/keda"))
	r.NoError(helmInstall(ns, "http-add-on", cfg.AddonChartLocation))

	t.Cleanup(func() {
		t.Logf("Cleaning up")
		r.NoError(helmDelete(ns, "http-add-on"))
		r.NoError(helmDelete(ns, "keda"))
		r.NoError(deleteNS(ns))
	})

}
