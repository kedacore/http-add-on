package e2e

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func setup(t *testing.T, ns string, cfg *config) {
	t.Helper()
	r := require.New(t)

	r.NoError(helmRepoAdd("kedacore", "https://kedacore.github.io/charts"))
	r.NoError(helmRepoUpdate())
	r.NoError(helmInstall(ns, "keda", "kedacore/keda"))
	r.NoError(helmInstall(ns, "http-add-on", cfg.AddonChartLocation))
	helmInstall(ns, "xkcd", cfg.ExampleAppChartLocation)
}

func teardown(t *testing.T, ns string) {
	t.Helper()
	r := require.New(t)
	t.Logf("Cleaning up")
	// always delete the charts in LIFO order
	r.NoError(helmDelete(ns, "xkcd"))
	r.NoError(helmDelete(ns, "http-add-on"))
	r.NoError(helmDelete(ns, "keda"))
	r.NoError(deleteNS(ns))
}
