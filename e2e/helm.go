package e2e

import (
	"fmt"

	"github.com/codeskyblue/go-sh"
)

func helmDelete(namespace, chartName string) error {
	return sh.Command(
		"helm",
		"delete",
		"-n",
		namespace,
		chartName,
	).Run()
}

func helmRepoAdd(name, url string) error {
	return sh.Command(
		"helm",
		"repo",
		"add",
		name,
		url,
	).Run()
}
func helmRepoUpdate() error {
	return sh.Command(
		"helm",
		"repo",
		"update",
	).Run()
}

func emptyHelmVars() map[string]string {
	return map[string]string{}
}
func helmInstall(
	namespace,
	chartName,
	chartLoc string,
	vars map[string]string,
) error {
	helmArgs := []string{
		"install",
		chartName,
		chartLoc,
		"-n",
		namespace,
		"--create-namespace",
	}
	for k, v := range vars {
		helmArgs = append(helmArgs, fmt.Sprintf(
			"--set %s=%s",
			k,
			v,
		))
	}
	return sh.Command(
		"helm",
		helmArgs,
	).Run()
}
