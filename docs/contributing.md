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

### KEDA

Follow the [install instructions](./install.md) to check out how to install and get this add-on up and running.

## Build scripts

This project uses [Mage](https://magefile.org) as opposed to Make because it's way faster to build and push images, as well as to run tests and other common tasks. Please install [version v1.11.0](https://github.com/magefile/mage/releases/tag/v1.11.0) or above to have access to the task runner.

### In the Root Directory

The Magefile located in the root directory has targets useful for the whole project. There is another magefile [in the operator directory](../operator/magefile.go), which has targets more specific to the operator module.

The most useful and common commands from the root directory are listed below. Please see the "In the Operator Directory" section for the operator-specific targets. Whther you're in the root or the operator directory, you can always run the following general helper commands:

- `mage -l`: shows a list of all available commands
- `mage -h <command>`: shows command-specific details
- `mage -h`: shows the general help

> All commands are case insensitive, so `buildAll` and `buildall` are the same.

- `mage build`: Builds all the binaries for local testing.
- `mage test`: Tests the entire codebase
- `mage dockerbuild`: Builds all docker images
  - Please see the below "Environment Variables" section for more information on this command
- `mage dockerpush`: Pushes all docker images, without building them first
  - Please see the below "Environment Variables" section for more information on this command

### In the Operator Directory

- `mage Manifests`: Builds all the manifest files for Kubernetes, it's important to build after every change
  to a Kustomize annotation.
- `mage All`: Generates the operator.

### Required Environment Variables

Some of the above commands require several environment variables to be set. You should set them once in your environment to ensure that you can run these targets. We recommend using [direnv](https://direnv.net) to set these environment variables once, so that you don't need to remember to do it.

- `KEDAHTTP_SCALER_IMAGE`: the fully qualified name of the [scaler](../scaler) image. This is used to build, push, and install the scaler into a Kubernetes cluster (required)
- `KEDAHTTP_INTERCEPTOR_IMAGE`: the fully qualified name of the [interceptor](../interceptor) image. This is used to build, push, and install the interceptor into a Kubernetes cluster (required)
- `KEDAHTTP_OPERATOR_IMAGE`: the fully qualified name of the [operator](../operator) image. This is used to build, push, and install the operator into a Kubernetes cluster (required)
- `KEDAHTTP_NAMESPACE`: the Kubernetes namespace to which to install the add on and other required components (optional, defaults to `kedahttp`)

>Suffic any `*_IMAGE` variable with `<keda-git-sha>` and the build system will automatically replace it with `sha-$(git rev-parse --short HEAD)`
