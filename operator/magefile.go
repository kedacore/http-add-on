//+build mage

package main

import (
	"context"
	"fmt"
	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

const (
	IMAGE_NAME    string = "keda-http-addon-controller:latest"
	CRD_OPTIONS   string = "crd:trivialVersions=true"
	RBAC_ROLENAME string = "keda-http-manager-role"
)

var CONTROLLER_GEN_PATH string = ""

func controllerGen(ctx context.Context) error {
	binaryName := "controller-gen"
	genPath, genErr := sh.Output("which", binaryName)
	if genErr != nil {
		sh.RunV("./scripts/download-controller-gen.sh")
		genPath, genErr = sh.Output("which", binaryName)
		if genErr != nil {
			return genErr
		}
	}
	CONTROLLER_GEN_PATH = genPath
	fmt.Printf("Controller gen path set to %q\n", CONTROLLER_GEN_PATH)
	return nil
}

func SetImage(ctx context.Context, image string) error {
	if image == "" {
		image = IMAGE_NAME
	}
	return sh.RunV("./scripts/set-kustomize-image.sh", image)
}

func Charts(ctx context.Context) error {
	mg.SerialDeps(Manifests, mg.F(SetImage, ""))
	return sh.RunV(
		"kustomize",
		"build",
		"config/default",
		">",
		"../charts/keda-http-operator/templates/keda-http-operator.yml",
	)
}

func Manager(ctx context.Context) {
	mg.SerialDeps(Generate, Fmt, Vet)
}

func Test(ctx context.Context) {
	mg.SerialDeps(Generate, Fmt, Vet, Manifests)
}

func Run(ctx context.Context) {
	mg.SerialDeps(Generate, Fmt, Vet, Manifests)
}

func Fmt() error {
	return sh.RunV("go", "fmt", "./...")
}

func Vet() error {
	return sh.RunV("go", "vet", "./...")
}

func Manifests() error {
	mg.SerialDeps(controllerGen)
	return sh.RunV(
		CONTROLLER_GEN_PATH,
		CRD_OPTIONS,
		fmt.Sprintf("rbac:roleName=%q", RBAC_ROLENAME),
		"webhook",
		fmt.Sprintf("paths=%q", "./..."),
		"output:src:artifacts:config=config/crd/bases",
	)
}

func Generate() error {
	mg.SerialDeps(controllerGen)
	return sh.RunV(
		CONTROLLER_GEN_PATH,
		fmt.Sprintf("object:headerFile=%q", "hack/boilerplate.go.txt"),
		fmt.Sprintf("paths=%q", "./..."),
	)
}

func All(ctx context.Context) {
	mg.Deps(Manager)
}
