//+build mage

package main

import (
	"context"
	"fmt"
	"github.com/magefile/mage/sh"
)

const (
	IMAGE_NAME  string = "keda-http-addon-controller:latest"
	CRD_OPTIONS string = "crd:trivialVersions=true"
)

func controllerGen(ctx context.Context) {
	output, err := sh.Output("which", "controller-gen")
	if err != nil {
		return err
	}
	fmt.Println(output)
}

func All(ctx context.Context) {
	mg.Deps(controllerGen)
}
