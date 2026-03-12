//go:build e2e

package default_test

import (
	"os"
	"testing"

	"sigs.k8s.io/e2e-framework/pkg/env"

	h "github.com/kedacore/http-add-on/test/helpers"
)

var testenv env.Environment

func TestMain(m *testing.M) {
	testenv = h.NewTestEnv()
	os.Exit(testenv.Run(m))
}
