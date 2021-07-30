package e2e

import "github.com/magefile/mage/sh"

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

func helmInstall(namespace, chartName, chartLoc string) error {
	return sh.RunV(
		"helm",
		"install",
		chartName,
		chartLoc,
		"-n",
		namespace,
		"--create-namespace",
	)
}
