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
make helm-upgrade-keda
```

>This document will rely on environment variables such as `${NAMESPACE}` to indicate a value you should customize and provide to the relevant command. In the above `helm install` command, `${NAMESPACE}` should be the namespace you'd like to install KEDA into. KEDA must be installed in every namespace that you'd like to host KEDA-HTTP powered apps.

## Install via Helm Chart

This repository is within KEDA's default helm repository on [kedacore/charts](http://github.com/kedacore/charts), you can install it by running:

```console
helm repo add kedacore https://kedacore.github.io/charts
helm repo update

helm install http-add-on kedacore/keda-add-ons-http --create-namespace --namespace keda
```

## Installing HTTP Components

This repository also comes with a Helm chart built in. To install the HTTP add on with sensible defaults, first check out this repository and `cd` into the root directory (if you haven't already):

```shell
git clone https://github.com/kedacore/http-add-on.git
cd http-add-on
```

Next, install the HTTP add on. The below command will install the add on if it doesn't already exist:

```shell
make helm-upgrade-operator
```

>Installing the HTTP add on won't affect any running workloads in your cluster. You'll need to install an `HTTPScaledObject` for each individual `Deployment` you want to scale. For more on how to do that, please see the [walkthrough](./walkthrough.md).

There are a few environment variables in the above command that you can set to customize how it behaves:

- `NAMESPACE` - which Kubernetes namespace to install KEDA-HTTP. This should be the same as where you installed KEDA itself (required)
- `OPERATOR_DOCKER_IMG` - the name of the operator's Docker image (optional - falls back to the latest release)
- `SCALER_DOCKER_IMG` - the name of the scaler's Docker image (optional - falls back to the latest release)
- `INTERCEPTOR_DOCKER_IMG` - the name of the interceptor's Docker image (optional - falls back to the latest release)

>I recommend using [direnv](https://direnv.net/) to store your environment variables

### If You're Installing into a [Microk8s](https://microk8s.io) Cluster

You might be working with a local development cluster like Microk8s, which offers a local, in-cluster registry. In this case, the previous `*_DOCKER_IMG` variables won't work for the Helm chart and you'll need to use a custom `helm` command such as the below:

```shell
helm upgrade kedahttp ./charts/keda-add-ons-http \
    --install \
    --namespace ${NAMESPACE} \
    --create-namespace \
    --set image=localhost:32000/keda-http-operator \
    --set images.scaler=localhost:32000/keda-http-scaler \
    --set images.interceptor=localhost:32000/keda-http-interceptor
```

In the above command, `localhost:32000` is the address of the registry from inside a Microk8s cluster.
