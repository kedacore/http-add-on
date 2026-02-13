# Developing

In this file you'll find all the references needed for you to start contributing code to the HTTP Add-on project.

## Getting started

To get started, first [fork](https://github.com/kedacore/http-add-on/fork) this repository to your account.
You'll need to have the following tools installed:

- [Go](https://go.dev/) for development
- [golangci-lint](https://golangci-lint.run/) for Go linting
- [ko](https://ko.build/) for building container images and deploying
- [kustomize](https://kubectl.docs.kubernetes.io/installation/kustomize/) for generating Kubernetes manifests
- [Make](https://www.gnu.org/software/make/) for build automation
- [pre-commit](https://pre-commit.com/) for static checks (_optional_)

## Setting up pre-commit hooks

Install [pre-commit](https://pre-commit.com/#install) and [golangci-lint](https://golangci-lint.run/welcome/install/), then register the git hooks:

```bash
pre-commit install --hook-type pre-commit --hook-type pre-push
```

This enables automatic static checks (formatting, linting, changelog validation, etc.) on `git commit` and `git push` as configured in `.pre-commit-config.yaml`.
To run all checks manually: `make pre-commit`.

## Prerequisites

### Kubernetes cluster

You'll need a running Kubernetes cluster.
You can use a cloud provider (AKS, GKE, EKS) or a local cluster like [KinD](https://kind.sigs.k8s.io/), [k3s](https://k3s.io/), or [Minikube](https://minikube.sigs.k8s.io/docs/).

### KEDA

Install KEDA and the HTTP Add-on following the [install instructions](./install.md).

## Makefile Reference

- `make build`: Build all binaries locally
- `make test`: Run linter and unit tests
- `make e2e-test-local`: Run e2e tests (assumes cluster is already set up)
- `make generate`: Generate code (DeepCopy) and Kubernetes manifests
- `make deploy`: Build and deploy all components to the cluster
- `make deploy-interceptor`: Build and deploy the interceptor
- `make deploy-operator`: Build and deploy the operator
- `make deploy-scaler`: Build and deploy the scaler
- `make pre-commit`: Run all static checks

## Local Development with ko

This project uses [ko](https://ko.build/) for building container images and deploying to Kubernetes.
ko builds Go binaries and packages them into container images without requiring Docker, with automatic dependency caching for fast incremental builds.

Set the `KO_DOCKER_REPO` environment variable for your target:

- **KinD:** `export KO_DOCKER_REPO=kind.local`
- **Local registry:** `export KO_DOCKER_REPO=localhost:5001`

After making code changes, deploy a single component:

```bash
make deploy-interceptor
```

Or deploy all components at once:

```bash
make deploy
```

ko will:

1. Build the Go binary with dependency caching
2. Create a container image with layer caching
3. Push to the configured registry
4. Apply manifests with resolved image references

## Debugging

To inspect the interceptor's pending request queue, port-forward the admin service and query the `/queue` endpoint:

```bash
kubectl port-forward -n keda svc/keda-add-ons-http-interceptor-admin 9090
curl localhost:9090/queue
```
