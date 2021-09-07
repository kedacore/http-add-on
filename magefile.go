//go:build mage
// +build mage

package main

import (
	"context"
	"fmt"

	"github.com/kedacore/http-add-on/pkg/build"
	"github.com/kedacore/http-add-on/pkg/env"
	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

// Note for Mac M1 users building Docker images:
// If you want to build images for Linux (like, for example, an
// AKS/GKE/EKS, DOKS cluster), you need to use docker's buildx driver
// to do so. This would be the command to build the interceptor for
// 64 Bit Amd and ARM platforms on Linux, for example:
//
//	docker buildx build --platform linux/amd64,linux/arm64 --push -t testingkeda.azurecr.io/interceptor:testing -f interceptor/Dockerfile .
//
// See
// https://blog.jaimyn.dev/how-to-build-multi-architecture-docker-images-on-an-m1-mac/
// for more details.

// Global consts
const (
	DEFAULT_NAMESPACE string = "kedahttp"

	ACR_REGISTRY_NAME         = "KEDAHTTP_ACR_REGISTRY"
	SCALER_IMAGE_ENV_VAR      = "KEDAHTTP_SCALER_IMAGE"
	INTERCEPTOR_IMAGE_ENV_VAR = "KEDAHTTP_INTERCEPTOR_IMAGE"
	OPERATOR_IMAGE_ENV_VAR    = "KEDAHTTP_OPERATOR_IMAGE"
	NAMESPACE_ENV_VAR         = "KEDAHTTP_NAMESPACE"
)

var goBuild = sh.OutCmd("go", "build", "-o")

type Scaler mg.Namespace

// Generate Go build of the scaler binary
func (Scaler) Build(ctx context.Context) error {
	fmt.Println("Running scaler binary build")
	out, err := goBuild("bin/scaler", "./scaler")
	if err != nil {
		return err
	}
	fmt.Println("Finished building scaler")
	fmt.Println("Command Output: ", out)

	return nil
}

// Run tests on the Scaler
func (Scaler) Test(ctx context.Context) error {
	fmt.Println("Running scaler tests")
	testOutput, err := sh.Output("go", "test", "./scaler/...")
	fmt.Println(testOutput)
	if err != nil {
		return err
	}

	return nil
}

func (Scaler) DockerBuild(ctx context.Context) error {
	img, err := build.GetImageName(SCALER_IMAGE_ENV_VAR)
	if err != nil {
		return err
	}
	return build.DockerBuild(img, "scaler/Dockerfile", ".")
}

// Build the scaler docker image using ACR tasks.
//
// This command reads the value of the following environment variables:
//
// - KEDAHTTP_ACR_REGISTRY - for the value of the --registry flag
// - KEDAHTTP_SCALER_IMAGE -- for the value of the --image flag
//
// it returns an error if either of the env vars are not set or they are and
// the build fails.
func (Scaler) DockerBuildACR(ctx context.Context) error {
	registry, err := env.Get(ACR_REGISTRY_NAME)
	if err != nil {
		return err
	}
	image, err := build.GetImageName(SCALER_IMAGE_ENV_VAR)
	if err != nil {
		return err
	}
	return build.DockerBuildACR(registry, image, "scaler/Dockerfile", ".")
}

func (Scaler) DockerPush(ctx context.Context) error {
	image, err := build.GetImageName(SCALER_IMAGE_ENV_VAR)
	if err != nil {
		return err
	}
	return build.DockerPush(image)
}

type Operator mg.Namespace

// Generate Go build of the operator binary
func (Operator) Build(ctx context.Context) error {
	fmt.Println("Running operator binary build")
	out, err := goBuild("bin/operator", "./operator")
	if err != nil {
		return err
	}
	fmt.Println("Finished building operator")
	fmt.Println("Command Output: ", out)

	return nil
}

// Run operator tests
func (Operator) Test(ctx context.Context) error {
	fmt.Println("Running operator tests")
	testOutput, err := sh.Output("go", "test", "./operator/...")
	if err != nil {
		return err
	}
	fmt.Println(testOutput)

	return nil
}

func (Operator) DockerBuild(ctx context.Context) error {
	img, err := build.GetImageName(OPERATOR_IMAGE_ENV_VAR)
	if err != nil {
		return err
	}
	return build.DockerBuild(img, "operator/Dockerfile", ".")
}

// Build the operator docker image using ACR tasks.
//
// This command reads the value of the following environment variables:
//
// - KEDAHTTP_ACR_REGISTRY - for the value of the --registry flag
// - KEDAHTTP_INTERCEPTOR_IMAGE -- for the value of the --image flag
//
// it returns an error if either of the env vars are not set or they are and
// the build fails.
func (Operator) DockerBuildACR(ctx context.Context) error {
	registry, err := env.Get(ACR_REGISTRY_NAME)
	if err != nil {
		return err
	}
	image, err := build.GetImageName(OPERATOR_IMAGE_ENV_VAR)
	if err != nil {
		return err
	}
	return build.DockerBuildACR(registry, image, "operator/Dockerfile", ".")
}

func (Operator) DockerPush(ctx context.Context) error {
	image, err := build.GetImageName(OPERATOR_IMAGE_ENV_VAR)
	if err != nil {
		return err
	}
	return build.DockerPush(image)
}

type Interceptor mg.Namespace

// Generate Go build of the interceptor binary
func (Interceptor) Build(ctx context.Context) error {
	fmt.Println("Running interceptor binary build")
	out, err := goBuild("bin/interceptor", "./interceptor")
	if err != nil {
		return err
	}
	fmt.Println("Finished building interceptor")
	fmt.Println("Command Output: ", out)

	return nil
}

// Run interceptor tests
func (Interceptor) Test(ctx context.Context) error {
	fmt.Println("Running interceptor tests")
	testOutput, err := sh.Output("go", "test", "./interceptor/...")
	if err != nil {
		return err
	}
	fmt.Println(testOutput)

	return nil
}

// DockerBuild builds the interceptor docker image. It looks for the
// KEDAHTTP_INTERCEPTOR_IMAGE environment variable and builds the image with
// that as the name
func (Interceptor) DockerBuild(ctx context.Context) error {
	image, err := build.GetImageName(INTERCEPTOR_IMAGE_ENV_VAR)
	if err != nil {
		return err
	}
	return build.DockerBuild(image, "interceptor/Dockerfile", ".")
}

// Build the interceptor docker image using ACR tasks.
//
// This command reads the value of the following environment variables:
//
// - KEDAHTTP_ACR_REGISTRY - for the value of the --registry flag
// - KEDAHTTP_INTERCEPTOR_IMAGE -- for the value of the --image flag
//
// it returns an error if either of the env vars are not set or they are and
// the build fails.
func (Interceptor) DockerBuildACR(ctx context.Context) error {
	registry, err := env.Get(ACR_REGISTRY_NAME)
	if err != nil {
		return err
	}
	image, err := build.GetImageName(INTERCEPTOR_IMAGE_ENV_VAR)
	if err != nil {
		return err
	}
	return build.DockerBuildACR(registry, image, "interceptor/Dockerfile", ".")
}

func (Interceptor) DockerPush(ctx context.Context) error {
	image, err := build.GetImageName(INTERCEPTOR_IMAGE_ENV_VAR)
	if err != nil {
		return err
	}
	return build.DockerPush(image)
}

// Build all binaries
func Build() {
	fmt.Println("Building all binaries")
	scaler := Scaler{}
	interceptor := Interceptor{}
	operator := Operator{}
	mg.Deps(scaler.Build, operator.Build, interceptor.Build)
}

// Run tests on all the components in this project
func Test() error {
	return sh.RunV("go", "test", "-timeout=20s", "./...")
}

// --- Docker --- //

// DockerBuild builds the operator, scaler and interceptor images in parallel
func DockerBuild(ctx context.Context) error {
	scaler, operator, interceptor := Scaler{}, Interceptor{}, Operator{}
	mg.Deps(scaler.DockerBuild, operator.DockerBuild, interceptor.DockerBuild)
	return nil
}

// DockerBuildACR builds the operator, scaler and interceptor images in parallel,
// all using ACR tasks
func DockerBuildACR(ctx context.Context) error {
	scaler, operator, interceptor := Scaler{}, Interceptor{}, Operator{}
	mg.Deps(scaler.DockerBuildACR, operator.DockerBuildACR, interceptor.DockerBuildACR)
	return nil
}

// Pushes a given image name to a given repository
func DockerPush(ctx context.Context) error {
	scaler, operator, interceptor := Scaler{}, Interceptor{}, Operator{}
	mg.Deps(scaler.DockerPush, operator.DockerPush, interceptor.DockerPush)
	return nil
}

// --- Helm --- //

// Upgrades or installs the Add-on onto the current cluster.
// Issuing "mage helmupgradeoperator kedahttp kedacore" will download
// "kedacore/http-add-on-operator:{SHA}" image and install along with the
// interceptor and scaler images on the same SHA
func UpgradeOperator(ctx context.Context) error {
	namespace, err := env.Get(NAMESPACE_ENV_VAR)
	if err != nil {
		namespace = DEFAULT_NAMESPACE
	}
	operatorImg, err := build.GetImageName(OPERATOR_IMAGE_ENV_VAR)
	if err != nil {
		return err
	}
	scalerImg, err := build.GetImageName(SCALER_IMAGE_ENV_VAR)
	if err != nil {
		return err
	}
	interceptorImg, err := build.GetImageName(INTERCEPTOR_IMAGE_ENV_VAR)
	if err != nil {
		return err
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
		fmt.Sprintf("images.operator=%s", operatorImg),
		"--set",
		fmt.Sprintf("images.scaler=%s", scalerImg),
		"--set",
		fmt.Sprintf("images.interceptor=%s", interceptorImg),
	); err != nil {
		return err
	}

	return nil
}

// Deletes the operator release
func DeleteOperator(ctx context.Context) error {
	namespace, err := env.Get(NAMESPACE_ENV_VAR)
	if err != nil {
		namespace = DEFAULT_NAMESPACE
	}
	if err := sh.RunV("helm", "delete", "-n", namespace, "kedahttp"); err != nil {
		return err
	}
	return nil
}

// Installs or upgrades KEDA in the given namespace
func InstallKeda(ctx context.Context) error {
	namespace, err := env.Get(NAMESPACE_ENV_VAR)
	if err != nil {
		namespace = DEFAULT_NAMESPACE
	}
	if err := sh.RunV(
		"helm",
		"upgrade",
		"kedacore/keda",
		"--install",
		"--namespace",
		namespace,
		"--create-namespace",
		"--set",
		fmt.Sprintf("watchNamespace=%s", namespace),
	); err != nil {
		return err
	}

	return nil
}

// Deletes the installed release of KEDA in the given namespaces
func DeleteKeda(ctx context.Context) error {
	namespace, err := env.Get(NAMESPACE_ENV_VAR)
	if err != nil {
		namespace = DEFAULT_NAMESPACE
	}
	if err := sh.RunV(
		"helm",
		"delete",
		"-n",
		namespace,
		"keda",
	); err != nil {
		return err
	}
	return nil
}

func InstallXKCD(ctx context.Context) error {
	namespace, err := env.Get(NAMESPACE_ENV_VAR)
	if err != nil {
		namespace = DEFAULT_NAMESPACE
	}
	if err := sh.RunV(
		"helm",
		"upgrade",
		"xkcd",
		"./examples/xkcd",
		"--install",
		"--namespace",
		namespace,
		"--create-namespace",
	); err != nil {
		return err
	}

	return nil
}

// --- Operator tasks --- //

// Generates the operator
func (Operator) Generate() error {
	if err := sh.RunV("mage", "-d", "operator", "all"); err != nil {
		return err
	}

	return nil
}

// Rebuilds all manifests for the operator
func (Operator) BuildManifests() error {
	if err := sh.RunV("mage", "-d", "operator", "manifests"); err != nil {
		return err
	}
	return nil
}

// --- Misc --- //

// Generates protofiles for external scaler
func (Scaler) GenerateProto() error {
	if err := sh.RunV(
		"protoc",
		"--go_out",
		".",
		"--go_opt",
		"paths=source_relative",
		"--go-grpc_out",
		".",
		"--go-grpc_opt",
		"paths=source_relative",
		"proto/scaler.proto",
	); err != nil {
		return err
	}

	return nil
}

// Create a new example HTTPScaledObject
func NewHTTPSO(ctx context.Context, namespace string) error {
	return sh.RunWithV(
		make(map[string]string),
		"kubectl", "create", "-f", "examples/httpscaledobject.yaml", "-n", namespace,
	)
}

func ShowHTTPSO(ctx context.Context, namespace string) error {
	return sh.RunWithV(
		make(map[string]string),
		"kubectl", "get", "httpscaledobject", "-n", namespace,
	)
}

// Delete the example HTTPScaledObject created from NewHTTPSO
func DeleteHTTPSO(ctx context.Context, namespace string) error {
	return sh.RunWithV(
		make(map[string]string),
		"kubectl", "delete", "httpscaledobject", "xkcd", "-n", namespace,
	)
}

func TestE2E(ctx context.Context) error {
	return sh.RunV("go", "test", "-test.v", "./e2e...")
}
