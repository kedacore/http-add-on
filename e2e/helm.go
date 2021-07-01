package e2e

import (
	"fmt"

	"github.com/magefile/mage/sh"
)

func helmDelete(namespace, chartName string) error {
	return sh.RunV(
		"helm",
		"delete",
		"-n",
		namespace,
		chartName,
	)
}

func helmRepoAdd(name, url string) error {
	return sh.RunV(
		"helm",
		"repo",
		"add",
		name,
		url,
	)
}
func helmRepoUpdate() error {
	return sh.RunV(
		"helm",
		"repo",
		"update",
	)
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
	return sh.RunV(
		"helm",
		helmArgs...,
	)
}
