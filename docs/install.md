# Installing the KEDA HTTP Add On

The HTTP Add On is highly module and, as expected, builds on top of KEDA core. Below are some additional components:

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
helm install keda kedacore/keda --namespace ${NAMESPACE} --create-namespace
```

>This document will rely on several variables, such as `${NAMESPACE}` to indicate a value you should customize and provide to the relevant command. In the above `helm install` command, `${NAMESPACE}` should be the namespace you'd like to install KEDA into. KEDA must be installed in every namespace that you'd like to host KEDA HTTP powered apps.

## Installing HTTP Components

This repository comes with a Helm chart built in. To install the HTTP add on with sensible defaults, first check out this repository and `cd` into the root directory (if you haven't already):

```shell
git clone https://github.com/kedacore/http-add-on.git
cd http-add-on
```

Next, run `helm install` to install the HTTP add on:

```shell
helm upgrade kedahttp ./charts/keda-http-operator \
    --install \
    -n ${NAMESPACE} \
    --create-namespace \
    --set image=arschles/keda-http-operator:$(git rev-parse --short HEAD)
```

>The above command will install KEDA HTTP if it doesn't already exist in the given `${NAMESPACE}`. Otherwise, it will upgrade it according to the chart.

