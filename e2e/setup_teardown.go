package e2e

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func setup(t *testing.T, ns string, cfg *config) {
	empty := emptyHelmVars()
	t.Helper()
	// use assert rather than require so that everything
	// gets run even if something fails
	a := assert.New(t)

	a.NoError(helmRepoAdd("kedacore", "https://kedacore.github.io/charts"))
	a.NoError(helmRepoUpdate())
	t.Logf("Installing KEDA")
	a.NoError(helmInstall(
		ns,
		"keda",
		"kedacore/keda",
		empty,
	))
	t.Logf("Installing HTTP addon")
	a.NoError(helmInstall(
		ns,
		"http-add-on",
		cfg.AddonChartLocation,
		cfg.httpAddOnHelmVars(),
	))
	t.Logf("Installing XKCD")
	a.NoError(helmInstall(
		ns,
		"xkcd",
		cfg.ExampleAppChartLocation,
		empty,
	))
}

func teardown(t *testing.T, ns string) {
	t.Helper()
	// use assert rather than require so that everything
	// gets run even if something fails
	a := assert.New(t)
	t.Logf("Cleaning up")
	// always delete the charts in LIFO order
	a.NoError(helmDelete(ns, "xkcd"))
	a.NoError(helmDelete(ns, "http-add-on"))
	a.NoError(helmDelete(ns, "keda"))
	a.NoError(deleteNS(ns))
}
