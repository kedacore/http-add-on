package build

import (
	"fmt"

	"github.com/magefile/mage/sh"
)

type Cmd = func(args ...string) (string, error)

// GoBuild runs the go build command with the -ldflags flag and
// the git sha of the current build
func GoBuild() (Cmd, error) {
	version, err := getGitSHA()
	if err != nil {
		return nil, err
	}
	return sh.OutCmd("go", "build", "-ldflags", fmt.Sprintf(`"-X github.com/kedacore/http-add-on/pkg/build.version=%s"`, version), "-o"), nil
}
