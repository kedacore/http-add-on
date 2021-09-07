# Developing

In this file you'll find all the references needed for you to start contributing code to the HTTP Add-on project.

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

>Suffix any `*_IMAGE` variable with `<keda-git-sha>` and the build system will automatically replace it with `sha-$(git rev-parse --short HEAD)`

## Helpful Tips

The below tips assist with debugging, introspecting, or observing the current state of a running HTTP addon installation. They involve making network requests to cluster-internal (i.e. `ClusterIP` `Service`s). 

There are generally two ways to communicate with these services.

### Use `kubectl proxy`

`kubectl proxy` establishes an authenticated connection to the Kubernetes API server, runs a local web server, and lets you execute REST API requests against `localhost` as if you were executing them against the Kubernetes API server.

To establish one, run the following command in a separate terminal window:

```shell
kubectl proxy -p 9898
```

>You'll keep this proxy running throughout all of your testing, so make sure you keep this terminal window open.

### Use a dedicated running pod

The second way to communicate with these services is almost the opposite as the previous. Instead of bringing the API server to you with `kubectl proxy`, you'll be creating an execution environment closer to the API server.

First, launch a container with an interactive shell in Kubernetes with the following command (substituting your namespace in for `$NAMESPACE`):

```shell
kubectl run -it alpine --image=alpine -n $NAMESPACE
```

Then, when you see a `curl` command below, replace the entire path up to and including the `/proxy/` segment with just the name of the service and its port. For example, `curl -L localhost:9898/api/v1/namespaces/$NAMESPACE/services/keda-add-ons-http-interceptor-admin:9090/proxy/routing_ping` would just become `curl -L keda-add-ons-http-interceptor-admin:9090/routing_ping`

### Routing Table - Interceptor

Any interceptor pod has both a _proxy_ and _admin_ server running inside it. The proxy server is where users send HTTP requests to, and the admin server is for internal use. You can use this server to

1. Prompt the interceptor to re-fetch the routing table from the interceptor, or
2. Print out the interceptor's current routing table (useful for debugging)

Assuming you've run `kubectl proxy` in a separate terminal window, prompt for a re-fetch with the below command (substitute `${NAMESPACE}` for your appropriate namespace):

Then, to prompt for a re-fetch (in a separate terminal shell):

```shell
curl -L localhost:9898/api/v1/namespaces/$NAMESPACE/services/keda-add-ons-http-interceptor-admin:9090/proxy/routing_ping
```

>To print out the current routing table without a re-fetch, replace `routing_ping` with `routing_table`

### Queue Counts - Interceptor

You can use the same interceptor port forward that you established in the previous section to fetch the HTTP pending queue counts table. This is the same table that the external scaler requests. See the "Queue Counts - Scaler" section below for more details on that.

To fetch the queue counts from an interceptor, ensure you've established a `kubectl proxy` on port 9898 and use the below `curl` command (again, substituting your preferred namespace for `$NAMESPACE`):

```shell
curl -L localhost:9898/api/v1/namespaces/$NAMESPACE/services/keda-add-ons-http-interceptor-admin:9090/proxy/queue
```

### Deployment Cache - Interceptor

You can use the same interceptor port forward that you established in the previous section to fetch a short summary of the state of its deployment cache (the data that it uses to determine whether and how long to hold requests prior to forwarding them). To do so, ensure that you've established a `kubectl proxy` on port 9898 and use the below `curl` command (again, substituting your preferred namespace for `$NAMESPACE`):

```shell
curl -L localhost:9898/api/v1/namespaces/$NAMESPACE/services/keda-add-ons-http-interceptor-admin:9090/proxy/deployments
```

The output of this command is a JSON map where the keys are the deployment name and the values are the latest known number of replicas for that deployment.

### Routing Table - Operator

The operator pod (whose name looks like `keda-add-ons-http-controller-manager-1234567`) has a similar `/routing_table` endpoint as the interceptor. That data returned from this endpoint, however, is the source of truth. Interceptors fetch their copies of the routing table from this endpoint. Accessing data from this endpoint is similar.

Ensure that you are running `kubectl proxy -p 9898` and then, in a separate terminal window, fetch the routing table from the operator with this `curl` command (again, substitute your namespace in for `${NAMESPACE}`):

```shell
curl -L localhost:9898/api/v1/namespaces/$NAMESPACE/services/keda-add-ons-http-operator-admin:9090/proxy/routing_table
```

### Queue Counts - Scaler

The external scaler fetches pending queue counts from each interceptor in the system, aggregates and stores them, and then returns them to KEDA when requested. KEDA fetches these data via the [standard gRPC external scaler interface](https://keda.sh/docs/2.3/concepts/external-scalers/#external-scaler-grpc-interface).

For convenience, the scaler also provides a plain HTTP server from which you can also fetch these metrics. 

Ensure that you are running `kubectl proxy -p 9898` and then, in a separate terminal window, fetch the routing table from the operator with this `curl` command (again, substitute your namespace in for `${NAMESPACE}`):

```shell
curl -L localhost:9898/api/v1/namespaces/$NAMESPACE/services/keda-add-ons-http-external-scaler:9091/proxy/queue
```

Or, you can prompt the scaler to fetch counts from all interceptors, aggregate, store, and return counts:

```shell
curl -L localhost:9898/api/v1/namespaces/$NAMESPACE/services/keda-add-ons-http-external-scaler:9091/proxy/queue_ping
```
