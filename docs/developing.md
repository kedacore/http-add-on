# Developing

In this file you'll find all the references needed for you to start contributing code to the HTTP Add-on project.

## Getting started

To get started, first [fork](https://github.com/kedacore/http-add-on/fork) this repository to your account. You'll need
to have the following tools installed:

- [Golang](http://golang.org/) for development
- [Docker](https://docker.com) for building the images and testing it locally
- [Pre-commit](https://pre-commit.com/) for static checks (_optional_)
- [golangci-lint](https://golangci-lint.run/) for Go linting

## Setting up pre-commit hooks

Install [pre-commit](https://pre-commit.com/#install) and [golangci-lint](https://golangci-lint.run/welcome/install/), then register the git hooks:

```bash
pre-commit install --hook-type pre-commit --hook-type pre-push
```

This enables automatic static checks (formatting, linting, changelog validation, etc.) on `git commit` and `git push` as configured in `.pre-commit-config.yaml`. To run all checks manually: `make pre-commit`.

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

### In the Root Directory

The Makefile located in the root directory has targets useful for the whole project.

> All commands are case sensitive.

- `make build`: Builds all the binaries for local testing
- `make test`: Run all unit tests
- `make e2e-test`: Run all e2e tests
- `make docker-build`: Builds all docker images
- `make docker-publish`: Build and push all Docker images
- `make publish-multiarch`: Build and push all Docker images for `linux/arm64` and `linux/amd64`
- `make manifests`: Generate all the manifest files for Kubernetes, it's important to build after every change
- `make deploy`: Deploys the HTTP Add-on to the cluster selected in `~/.kube/config` using `config` folder manifests
- `make pre-commit`: Execute static checks

### Required Environment Variables

Some of the above commands support changes in the default values:

- `IMAGE_REGISTRY`: Image registry to be used for docker images
- `IMAGE_REPO`: Repository to be used for docker images
- `VERSION`: Tag to be used for docker images
- `BUILD_PLATFORMS`: Built platform targeted for multi-arch docker images

## Debugging and Observing Components

The below tips assist with debugging, introspecting, or observing the current state of a running HTTP add-on installation. They involve making network requests to cluster-internal (i.e. `ClusterIP` `Service`s).

There are generally two ways to communicate with these services. In the following sections, we'll assume you are using the `kubectl proxy` method, but the most instructions will be simple enough to adapt to other methods.

We'll also assume that you have set the `$NAMESPACE` environment variable in your environment to the namespace in which the HTTP add-on is installed.

### Use `kubectl proxy`

`kubectl proxy` establishes an authenticated connection to the Kubernetes API server, runs a local web server, and lets you execute REST API requests against `localhost` as if you were executing them against the Kubernetes API server.

To establish one, run the following command in a separate terminal window:

```console
kubectl proxy -p 9898
```

>You'll keep this proxy running throughout all of your testing, so make sure you keep this terminal window open.

### Use a dedicated running pod

The second way to communicate with these services is almost the opposite as the previous. Instead of bringing the API server to you with `kubectl proxy`, you'll be creating an execution environment closer to the API server.

First, launch a container with an interactive console in Kubernetes with the following command (substituting your namespace in for `$NAMESPACE`):

```console
kubectl run -it alpine --image=alpine -n $NAMESPACE
```

Then, when you see a `curl` command below, replace the entire path up to and including the `/proxy/` segment with just the name of the service and its port. For example, `curl -L localhost:9898/api/v1/namespaces/$NAMESPACE/services/keda-add-ons-http-interceptor-admin:9090/proxy/routing_ping` would just become `curl -L keda-add-ons-http-interceptor-admin:9090/routing_ping`

### Interceptor

Any interceptor pod has both a _proxy_ and _admin_ server running inside it. The proxy server is where users send HTTP requests to, and the admin server is for internal use. The admin server runs on a separate port, fronted by a separate `Service`.

The admin server also performs following tasks:

1. Prompt the interceptor to re-fetch the routing table, or
2. Print out the interceptor's current routing table (useful for debugging)

#### Configuration

Run the following `curl` command to get the running configuration of the interceptor:

```console
curl -L localhost:9898/api/v1/namespaces/$NAMESPACE/services/keda-add-ons-http-interceptor-admin:9090/proxy/config
```

#### Routing Table

To prompt the interceptor to fetch the routing table, then print it out:

```console
curl -L localhost:9898/api/v1/namespaces/$NAMESPACE/services/keda-add-ons-http-interceptor-admin:9090/proxy/routing_ping
```

Or, to just ask the interceptor to print out its routing table:

```console
curl -L localhost:9898/api/v1/namespaces/$NAMESPACE/services/keda-add-ons-http-interceptor-admin:9090/proxy/routing_table
```

#### Queue Counts

To fetch the state of an individual interceptor's pending HTTP request queue:

```console
curl -L localhost:9898/api/v1/namespaces/$NAMESPACE/services/keda-add-ons-http-interceptor-admin:9090/proxy/queue
```

#### Deployment Cache

To fetch the current state of an individual interceptor's deployment queue:

```console
curl -L localhost:9898/api/v1/namespaces/$NAMESPACE/services/keda-add-ons-http-interceptor-admin:9090/proxy/deployments
```

The output of this command is a JSON map where the keys are the deployment name and the values are the latest known number of replicas for that deployment.

### Operator

Like the interceptor, the operator has an admin server that has HTTP endpoints against which you can run `curl` commands.

#### Configuration

Run the following `curl` command to get the running configuration of the operator:

```console
curl -L localhost:9898/api/v1/namespaces/$NAMESPACE/services/keda-add-ons-http-operator-admin:9090/proxy/config
```

#### Routing Table

The operator has a similar `/routing_table` endpoint as the interceptor. That data returned from this endpoint, however, is the source of truth. Interceptors fetch their copies of the routing table from this endpoint. Accessing data from this endpoint is similar.

Fetch the operator's routing table with the following command:

```console
curl -L localhost:9898/api/v1/namespaces/$NAMESPACE/services/keda-add-ons-http-operator-admin:9090/proxy/routing_table
```

### Scaler

Like the interceptor, the scaler has an HTTP admin interface against which you can run `curl` commands.

#### Configuration

Run the following `curl` command to get the running configuration of the scaler:

```console
curl -L localhost:9898/api/v1/namespaces/$NAMESPACE/services/keda-add-ons-http-external-scaler:9091/proxy/config
```

#### Queue Counts

The external scaler fetches pending queue counts from each interceptor in the system, aggregates and stores them, and then returns them to KEDA when requested. KEDA fetches these data via the [standard gRPC external scaler interface](https://keda.sh/docs/2.3/concepts/external-scalers/#external-scaler-grpc-interface).

For convenience, the scaler also provides a plain HTTP server from which you can also fetch these metrics. Fetch the queue counts from this HTTP server with the following command:

```console
curl -L localhost:9898/api/v1/namespaces/$NAMESPACE/services/keda-add-ons-http-external-scaler:9091/proxy/queue
```

Alternatively, you can prompt the scaler to fetch counts from all interceptors, aggregate, store, and return counts:

```console
curl -L localhost:9898/api/v1/namespaces/$NAMESPACE/services/keda-add-ons-http-external-scaler:9091/proxy/queue_ping
```

[Go back to landing page](./)
