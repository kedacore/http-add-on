package build

import (
	"fmt"
	"strings"

	"github.com/kedacore/http-add-on/pkg/env"
	"github.com/magefile/mage/sh"
)

const (
	gitShaSuffix = "<keda-git-sha>"
)

func getGitSHA() (string, error) {
	return sh.Output("git", "rev-parse", "--short", "HEAD")
}

// DockerBuild calls the following and returns the resulting error, if any:
//
//	docker build -t <image> -f <dockerfileLocation> <context>
func DockerBuild(image, dockerfileLocation, context string) error {
	return sh.RunV(
		"docker",
		"build",
		"-t",
		image,
		"-f",
		dockerfileLocation,
		context,
	)
}

func DockerBuildACR(registry, image, dockerfileLocation, context string) error {
	return sh.RunV(
		"az",
		"acr",
		"build",
		"--image",
		image,
		"--registry",
		registry,
		"--file",
		dockerfileLocation,
		context,
	)
}

// DockerPush calls the following and returns the resulting error, if any:
//
//	docker push <image>
func DockerPush(image string) error {
	return sh.RunV("docker", "push", image)
}

func GetImageName(envName string) (string, error) {
	img, err := env.Get(envName)
	if err != nil {
		return "", err
	}
	if strings.HasSuffix(img, gitShaSuffix) {
		sha, err := getGitSHA()
		if err != nil {
			return "", err
		}
		fmt.Println("using sha ", sha)
		trimmed := strings.TrimSuffix(img, gitShaSuffix)
		img = fmt.Sprintf("%ssha-%s", trimmed, sha)
	}
	return img, nil
}
