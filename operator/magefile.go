//+build mage

package main

import (
	"context"
	"fmt"
	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
	"os"
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

// Sets the image name for all manifests
func SetImage(ctx context.Context, image string) error {
	if image == "" {
		image = IMAGE_NAME
	}
	return sh.RunV("./scripts/set-kustomize-image.sh", image)
}

// Creates and copies the chart to the chart directory
func charts(ctx context.Context) error {
	mg.SerialDeps(Manifests, mg.F(SetImage, ""))
	chart, err := sh.Output(
		"kustomize",
		"build",
		"config/default",
	)
	if err != nil {
		return err
	}

	file, fErr := os.Create("../charts/keda-http-operator/templates/keda-http-operator.yml")
	if fErr != nil {
		return fErr
	}
	defer file.Close()

	if _, wErr := file.WriteString(chart); wErr != nil {
		return wErr
	}

	return nil
}

// Generates and formats the manager
func Manager(ctx context.Context) {
	mg.SerialDeps(Generate, Fmt, Vet)
}

// Runs go tests
func Test(ctx context.Context) {
	mg.SerialDeps(Generate, Fmt, Vet, Manifests)
}

// Runs the manager
func Run(ctx context.Context) error {
	mg.SerialDeps(Generate, Fmt, Vet, Manifests)
	return sh.RunV("go", "run", "main.go")
}

// Runs go fmt in this directory
func Fmt() error {
	return sh.RunV("go", "fmt", "./...")
}

// Runs go vet in this directory
func Vet() error {
	return sh.RunV("go", "vet", "./...")
}

// Builds all the manifest files
func Manifests() error {
	mg.SerialDeps(controllerGen)
	return sh.RunV(
		CONTROLLER_GEN_PATH,
		CRD_OPTIONS,
		fmt.Sprintf("rbac:roleName=%q", RBAC_ROLENAME),
		"webhook",
		fmt.Sprintf("paths=%q", "./..."),
		"output:crd:artifacts:config=config/crd/bases",
	)
}

// Generates the controller
func Generate() error {
	mg.SerialDeps(controllerGen)
	return sh.RunV(
		CONTROLLER_GEN_PATH,
		fmt.Sprintf("object:headerFile=%q", "hack/boilerplate.go.txt"),
		fmt.Sprintf("paths=%q", "./..."),
	)
}

// Creates the manager
func All(ctx context.Context) {
	mg.Deps(Manager)
}
