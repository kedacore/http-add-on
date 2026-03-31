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
- `make test`: Run unit tests
- `make lint`: Run linter
- `make lint-fix`: Run linter with auto-fix
- `make e2e-test`: Run all e2e tests against an existing cluster
- `make generate`: Generate code (DeepCopy) and Kubernetes manifests (run after modifying CRD types or webhook configs)
- `make deploy`: Build and deploy all components to the cluster
- `make deploy-interceptor`: Build and deploy the interceptor
- `make deploy-operator`: Build and deploy the operator
- `make deploy-scaler`: Build and deploy the scaler
- `make pre-commit`: Run all static checks

## Local Development with ko

This project uses [ko](https://ko.build/) for building container images and deploying to Kubernetes.
ko builds Go binaries and packages them into container images without requiring Docker, with automatic dependency caching for fast incremental builds.

Set the `KO_DOCKER_REPO` environment variable for your target:

- **Local registry:** `export KO_DOCKER_REPO=localhost:<port>`
- **KinD:** `export KO_DOCKER_REPO=kind.local` (only works with Docker, not Podman)

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

## Running E2E Tests

E2E tests live in `test/e2e/` and are organized by **profile**. Each profile groups tests that require a specific HTTP Add-on configuration.
For example, the `default` profile covers standard routing and scaling behavior with the default configuration, while the `tls` profile tests TLS termination and requires the interceptor to be deployed with TLS certificates enabled.

### Cluster setup

```bash
# Create a KinD cluster
kind create cluster

# Install all e2e dependencies and deploy the HTTP Add-on
make e2e-setup

# Or install individual dependencies (see Makefile for all targets)
make e2e-deps-otel-collector
make e2e-deps-jaeger
```

### Running tests

```bash
# Run all e2e tests (all profiles)
make e2e-test

# Run a specific profile
make e2e-test PROFILE=tls

# Run a specific test by name
make e2e-test PROFILE=default RUN=TestColdStart

# Run tests matching specific labels
make e2e-test E2E_ARGS="--labels=area=scaling"

# List tests without executing them
make e2e-test E2E_ARGS="--dry-run"
```

The `PROFILE` variable selects a test profile directory under `test/e2e/` (e.g. `PROFILE=tls` runs `./test/e2e/tls/...`). Each subdirectory in `test/e2e/` is a profile.
The `RUN` variable filters tests by name using Go's `-run` flag (supports regex, e.g. `RUN=TestColdStart` or `RUN="TestHost|TestPath"`).
The `E2E_ARGS` variable passes flags to the [e2e-framework](https://github.com/kubernetes-sigs/e2e-framework) via `-args` (e.g. `--labels`, `--feature`, `--skip-labels`, `--dry-run`).
