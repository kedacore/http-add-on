//+build mage

package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

// Global consts
const (
	BASE_PACKAGE_NAME string = "http-add-on"
)

// Enum types
type ModuleName string

const (
	SCALER      ModuleName = "scaler"
	INTERCEPTOR ModuleName = "interceptor"
	OPERATOR    ModuleName = "operator"
)

// --- Utils --- //

func getGitSHA() (string) {
	output, _ := sh.Output("git", "rev-parse", "--short", "HEAD")

	return output
}

func isValidModule(s string) error {
	b := (ModuleName)(s)
	switch (b) {
	case SCALER, OPERATOR, INTERCEPTOR:
		return nil
	}
	return errors.New(fmt.Sprintf("Invalid image name %q", s))
}

func getSlash(repository string) string {
	if repository == "" {
		return ""
	}
	return "/"
}

func getFullImageName(repository string, module string) string {
	return fmt.Sprintf(
		"%s%s%s-%v:%s",
		repository,
		getSlash(repository),
		BASE_PACKAGE_NAME,
		module,
		getGitSHA(),
	)
}

// --- Go Builds --- //

var goBuild = sh.OutCmd("go", "build", "-o")

// Generate Go build of the scaler binary
func BuildScaler(ctx context.Context) error {
	fmt.Println("Running scaler binary build")
	out, err := goBuild("bin/scaler", "./scaler")
	if err != nil {
		return err
	}
	fmt.Println("Finished building scaler")
	fmt.Println("Command Output: ", out)

	return nil
}

// Generate Go build of the operator binary
func BuildOperator(ctx context.Context) error {
	fmt.Println("Running operator binary build")
	out, err := goBuild("bin/operator", "./operator")
	if err != nil {
		return err
	}
	fmt.Println("Finished building operator")
	fmt.Println("Command Output: ", out)

	return nil
}

// Generate Go build of the interceptor binary
func BuildInterceptor(ctx context.Context) error {
	fmt.Println("Running interceptor binary build")
	out, err := goBuild("bin/interceptor", "./interceptor")
	if err != nil {
		return err
	}
	fmt.Println("Finished building interceptor")
	fmt.Println("Command Output: ", out)

	return nil
}

// Build all binaries
func BuildAll() {
	fmt.Println("Building all binaries")
	mg.Deps(BuildScaler, BuildOperator, BuildInterceptor)
}

// --- Docker --- //

// Builds a docker image specified by the name argument with the repository prefix
func DockerBuild(ctx context.Context, repository string, name string) error {
	if err := isValidModule(name); err != nil {
		return err
	}

	fmt.Println(fmt.Sprintf(
		"Running docker build for image %q",
		getFullImageName(repository, name),
	))

	err := sh.RunV(
		"docker",
		"build",
		"-t",
		getFullImageName(repository, name),
		"-f",
		fmt.Sprintf("%s/Dockerfile", name),
		".",
	)
	if err != nil {
		return err
	}

	fmt.Println(fmt.Sprintf("Finished building %q", getFullImageName(repository, name)))
	return nil
}

// Pushes a given image name to a given repository
func DockerPush(ctx context.Context, repository string, name string) error {
	if err := isValidModule(name); err != nil {
		return err
	}

	fmt.Println(fmt.Sprintf(
		"Running docker push for image %q",
		getFullImageName(repository, name),
	))

	err := sh.RunV(
		"docker",
		"push",
		getFullImageName(repository, name),
	)
	if err != nil {
		return err
	}

	fmt.Println(fmt.Sprintf("Finished pushing %q", getFullImageName(repository, name)))
	return nil
}

// Builds all the images to the given repository
func DockerBuildAll(repository string) {
	var fns []interface{}
	for _, module := range []ModuleName{SCALER, OPERATOR, INTERCEPTOR} {
		fns = append(fns, mg.F(DockerBuild, repository, (string)(module)))
	}
	mg.Deps(fns...)
}

// Pushes all the images to the given repository
func DockerPushAll(repository string) {
	var fns []interface{}
	for _, module := range []ModuleName{SCALER, OPERATOR, INTERCEPTOR} {
		fns = append(fns, mg.F(DockerPush, repository, (string)(module)))
	}
	mg.Deps(fns...)
}

// --- Helm --- //

// Upgrades or installs the Add-on onto the current cluster.
// Issuing "mage helmupgradeoperator kedahttp kedacore" will download
// "kedacore/http-add-on-operator:{SHA}" image and install along with the
// interceptor and scaler images on the same SHA
func UpgradeOperator(namespace string, imageRepository string) error {
	if namespace == "" {
		namespace = "kedahttp"
	}

	if err := sh.RunV(
		"helm",
		"upgrade",
		"kedahttp",
		"./charts/keda-http-operator",
		"--install",
		"--namespace",
		namespace,
		"--create-namespace",
		"--set",
		fmt.Sprintf("image=%s", getFullImageName(imageRepository, "operator")),
		"--set",
		fmt.Sprintf("images.scaler=%s", getFullImageName(imageRepository, "scaler")),
		"--set",
		fmt.Sprintf("images.interceptor=%s", getFullImageName(imageRepository, "interceptor")),
	); err != nil {
		return err
	}

	return nil
}

func DeleteOperator(namespace string) error {
	if namespace == "" {
		namespace = "kedahttp"
	}
	if err := sh.RunV("helm", "delete", "-n", namespace, "kedahttp"); err != nil {
		return err
	}
	return nil
}
