# Installing the KEDA HTTP Add On

The HTTP Add On is highly modular and, as expected, builds on top of KEDA core. Below are some additional components:

- **Operator** - watches for `ScaledHTTPObject` CRD resources and creates necessary backing Kubernetes resources (e.g. `Deployment`s, `Service`s, `ScaledObject`s, and so forth)
- **Scaler** - communicates scaling-related metrics to KEDA. By default, the operator will install this for you as necessary.
- **Interceptor** - a cluster-internal proxy that proxies incoming HTTP requests, communicating HTTP queue size metrics to the scaler, and holding requests in a temporary request queue when there are not yet any available app `Pod`s ready to serve. By default, the operator will install this for you as necessary.

>There is [pending work in KEDA](https://github.com/kedacore/keda/issues/615) that will eventually make this component optional. See [issue #6 in this repository](https://github.com/kedacore/http-add-on/issues/6) for even more background

## Installing KEDA

Before you install any of these components, you need to install KEDA. Below are simplified instructions for doing so with [Helm](https://helm.sh), but if you need anything more customized, please see the [official KEDA deployment documentation](https://keda.sh/docs/2.0/deploy/).

>If you need to install Helm, refer to the [installation guide](https://helm.sh/docs/intro/install/)

```shell
helm repo add kedacore https://kedacore.github.io/charts
helm repo update
helm install keda kedacore/keda --namespace ${NAMESPACE} --set watchNamespace=${NAMESPACE}
```

>This document will rely on environment variables such as `${NAMESPACE}` to indicate a value you should customize and provide to the relevant command. In the above `helm install` command, `${NAMESPACE}` should be the namespace you'd like to install KEDA into. KEDA must be installed in every namespace that you'd like to host KEDA-HTTP powered apps.

## Installing HTTP Components

This repository comes with a Helm chart built in. To install the HTTP add on with sensible defaults, first check out this repository and `cd` into the root directory (if you haven't already):

```shell
git clone https://github.com/kedacore/http-add-on.git
cd http-add-on
```

Next, install the HTTP add on. The below command will install the add on if it doesn't already exist:

```shell
helm install kedahttp ./charts/keda-http-operator -n ${NAMESPACE}
```

>Installing the HTTP add on won't affect any running workloads in your cluster. You'll need to install an `HTTPScaledObject` for each individual `Deployment` you want to scale. For more on how to do that, please see the [walkthrough](./walkthrough.md).

There are a few values that you can pass to the above `helm install` command by including `--set NAME=VALUE` on the command line.

- `images.operator` - the name of the operator's Docker image.
  - The default is `ghcr.io/kedacore/http-add-on-operator:latest`, which maps to the latest release at [github.com/kedacore/http-add-on/releases](https://github.com/kedacore/http-add-on/releases)
- `images.scaler` - the name of the scaler's Docker image.
  - The default is The default is `ghcr.io/kedacore/http-add-on-scaler:latest`, which maps to the latest release at [github.com/kedacore/http-add-on/releases](https://github.com/kedacore/http-add-on/releases)
- `images.interceptor` - the name of the interceptor's Docker image.
  - The default is `ghcr.io/kedacore/http-add-on-interceptor:latest`, which maps to the latest release at [github.com/kedacore/http-add-on/releases](https://github.com/kedacore/http-add-on/releases)
  
### A Note for Developers and Local Cluster Users

Local clusters like [Microk8s](https://microk8s.io/) offer in-cluster image registries. These are popular tools to speed up and ease local development. If you use such a tool for local development, we recommend that you use and push your images to its local registry. When you do, you'll want to set your `images.*` variables to the address of the local registry. In the case of MicroK8s, that address is `localhost:32000` and the `helm install` command would look like the following:

```shell
helm install kedahttp ./charts/keda-http-operator \
    -n ${NAMESPACE} \
    --set images.operator=localhost:32000/keda-http-operator \
    --set images.scaler=localhost:32000/keda-http-scaler \
    --set images.interceptor=localhost:32000/keda-http-interceptor
```

## Next Steps

Now that you're finished installing KEDA and the HTTP Add On, please proceed to the [walkthrough](./walkthrough.md) to test out your new installation.
