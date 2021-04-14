# Contributing

In this file you'll find all the references needed for you to start contributing with the HTTP Add-on project.

## Getting started

To get started, first [fork](https://github.com/kedacore/http-add-on/fork) this repository to your account. You'll need
to have the following tools installed:

- [Golang](http://golang.org/) for development
- [Docker](https://docker.com) for building the images and testing it locally

## Prerequisites

### Kubernetes cluster

It's recommended to have a running Kubernetes cluster to test the development, there are faster approaches using public
clouds like:

- Azure with [AKS](https://azure.microsoft.com/services/kubernetes-service/?WT.mc_id=opensource-12724-ludossan)
- Google Cloud with [GKE](https://cloud.google.com/kubernetes-engine)
- AWS with [EKS](https://aws.amazon.com/eks/)
- [Digital Ocean](https://www.digitalocean.com/products/kubernetes/)

These providers will let you deploy a simple and quick K8S cluster, however, they're paid. If you don't want to pay for
the service, you can host your own with a series of amazing tools like:

- [Microk8s](https://microk8s.io/)
- [Minikube](https://minikube.sigs.k8s.io/docs/)
- [K3S](https://k3s.io/)
- [KinD (Kubernetes in Docker)](https://kind.sigs.k8s.io/)

### Keda

Follow the [install instructions](./install.md) to check out how to install and get this add-on up and running.

## Build scripts

This project uses [Mage](https://magefile.org) as opposed to Make because it's way faster to build and push images, as well as to run tests and other common tasks. Please install [version v1.11.0](https://github.com/magefile/mage/releases/tag/v1.11.0) or above to have access to the task runner.

> **Note:** The Magefile located in the root directory is related to the whole project, so it gives you the ability to control the build and install process of all the modules in this project. On the other hand, the build binary located in the [operator](../operator/magefile.go) directory, is **just related to the operator module**.

The usage is as follows:

- Type `mage -l` on the magefile directory to print a list of all available commands
- Type `mage -h <command>` to check the help for that specific command
- `mage -h` shows the general help

Most of the commands are simple, and we have a few commands that chain other commands together, for reference on chains,
check the [Magefile](../magefile.go) source code. Below is a list of the most common build commands

> All commands are case insensitive, so `buildAll` and `buildall` are the same.

In the root directory:

- `mage All`: Builds all the binaries for local testing.
- `mage deleteOperator [namespace]`: Deletes the installed add-on in the given `namespace` for the active K8S
  cluster.
- `mage dockerBuildAll <repository>`: Builds all the images for the `interceptor`, `scaler`, and `operator` modules
  for the specified `repository`.
  - You can also build specific images by using `mage dockerBuild <repository> <module>`, where module is one
    of `interceptor`, `scaler`, or `operator`.
- `mage dockerPushAll <repository>`: Pushes all the built images for a given repository.
  - You can push the images using `mage dockerPush <repository> <module>` like the `dockerBuild` command.
- `mage installKeda [namespace]`: will install KEDA on the given namespace.
- `mage upgradeOperator [namespace] <image>`: Will install the add-on in the given `namespace` if not installed, or
  update it using the provided `image`.
- `mage Operator`: Alias to `mage -d operator All`, just to have everything on the same dir level.
- `mage Manifests`: Alias to `mage -d operator Manifests`.

> The default values for the `namespace` if not provided (when passed as `""`, like `mage upgradeOperator "" image`) is `kedahttp`

In the operator directory:

- `mage Manifests`: Builds all the manifest files for Kubernetes, it's important to build after every change
  to a Kustomize annotation.
- `mage All`: Generates the operator.
