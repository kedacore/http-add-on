# Getting Started With The HTTP Add On

After you've installed KEDA and the HTTP Add On (this project, we'll call it the "add on" for short), this document will show you how to get started with an example app.

If you haven't installed KEDA and the HTTP Add On (this project), please do so first. Follow instructions [install.md](./install.md) to complete your installation. Before you continue, make sure that you have your `NAMESPACE` environment variable set to the same value as it was when you installed.

## Creating An Application

You'll need to install a `Deployment` and `Service` first. You'll tell the add on to begin scaling it up and down after this step. Use this [Helm](https://helm.sh) command to create the resources you need:

```shell
helm install xkcd ./charts/xkcd -n ${NAMESPACE}
```

>To remove the app, run `helm delete xkcd -n ${NAMESPACE}`

## Creating an `HTTPScaledObject`

You interact with the operator via a CRD called `HTTPScaledObject`. This CRD object points the To get an example app up and running, read the notes below and then run the subsequent command from the root of this repository.

```shell
kubectl create -f -n $NAMESPACE examples/httpscaledobject.yaml
```

>If you'd like to learn more about this object, please see the [`HTTPScaledObject` reference](./ref/http_scaled_object.md).

## Testing Your Installation

You've now installed a web application and activated autoscaling by creating an `HTTPScaledObject` for it. For autoscaling to work properly, HTTP traffic needs to route through the `Service` that the add on has set up. You can use `kubectl port-forward` to quickly test things out:

```shell
k port-forward svc/xkcd-interceptor-proxy -n ${NAMESPACE} 8080:80 
```

### Routing to the Right `Service`

As said above, you need to route your HTTP traffic to the `Service` that the add on has created. If you have existing systems - like an ingress controller - you'll need to anticipate the name of these created `Service`s. Each one will be named consistently like so, in the same namespace as the `HTTPScaledObject` and your application (i.e. `$NAMESPACE`):

```shell
<deployment name>-interceptor-proxy
```

>The service will always be a `ClusterIP` type and will be created in the same namespace as the `HTTPScaledObject` you created.
