# Getting Started With The HTTP Add On

After you've installed KEDA and the HTTP Add On (this project, we'll call it the "add on" for short), this document will show you how to get started with an example app.

If you haven't installed KEDA and the HTTP Add On (this project), please do so first. Follow instructions [install.md](./install.md) to complete your installation.

>Before you continue, make sure that you have your `NAMESPACE` environment variable set to the same value as it was when you installed.

## Creating An Application

You'll need to install a `Deployment` and `Service` first. You'll tell the add on to begin scaling it up and down after this step. We've provided a [Helm](https://helm.sh) chart in this repository that you can use to try it out. Use this command to create the resources you need.

```shell
helm install xkcd ./examples/xkcd -n ${NAMESPACE}
```

You'll need to clone the repository to get access to this chart. If you have your own `Deployment` and `Service` installed, you can go right to creating an `HTTPScaledObject` in the next section.

>To remove the app, run `helm delete xkcd -n ${NAMESPACE}`

## Creating an `HTTPScaledObject`

You interact with the operator via a CRD called `HTTPScaledObject`. This CRD object points the To get an example app up and running, read the notes below and then run the subsequent command from the root of this repository.

```shell
kubectl create -f -n $NAMESPACE examples/v0.0.2/httpscaledobject.yaml
```

>If you'd like to learn more about this object, please see the [`HTTPScaledObject` reference](./ref/v0.2.0/http_scaled_object.md).

## Testing Your Installation

You've now installed a web application and activated autoscaling by creating an `HTTPScaledObject` for it. For autoscaling to work properly, HTTP traffic needs to route through the `Service` that the add on has set up. You can use `kubectl port-forward` to quickly test things out:

```shell
kubectl port-forward svc/keda-add-ons-http-interceptor-proxy -n ${NAMESPACE} 8080:80
```

### Routing to the Right `Service`

As said above, you need to route your HTTP traffic to the `Service` that the add on has created. If you have existing systems - like an ingress controller - you'll need to anticipate the name of these created `Service`s. Each one will be named consistently like so, in the same namespace as the `HTTPScaledObject` and your application (i.e. `$NAMESPACE`):

```shell
keda-add-ons-http-interceptor-proxy
```

>This is installed by the [Helm chart](https://github.com/kedacore/charts/tree/master/http-add-on) as a `ClusterIP` `Service` by default.

#### Installing and Using the [ingress-nginx](https://kubernetes.github.io/ingress-nginx/deploy/#using-helm) Ingress Controller

As mentioned above, the `Service` that the add-on creates will be inaccessible over the network from outside of your Kubernetes cluster.

While you can access it via the `kubectl port-forward` command above, we recommend against using that in a production setting. Instead, we recommend that you use an [ingress controller](https://kubernetes.io/docs/concepts/services-networking/ingress-controllers/) to route to the interceptor service. This section describes how to set up and use the NGINX Ingress controller.

First, install the controller using the commands below. These commands use Helm v3. For other installation methods, see the [installation page](https://kubernetes.github.io/ingress-nginx/deploy/).

```shell
helm repo add ingress-nginx https://kubernetes.github.io/ingress-nginx
helm repo update
helm install ingress-nginx ingress-nginx/ingress-nginx -n ${NAMESPACE}
```

An [`Ingress`](https://kubernetes.io/docs/concepts/services-networking/ingress/) resource was already created as part of the [xkcd chart](../examples/xkcd/templates/ingress.yaml), so the installed NGINX ingress controller will initialize, detect the `Ingress`, and begin routing to the xkcd interceptor `Service`.

When you're ready, please run `kubectl get svc -n ${NAMESPACE}`, find the `ingress-nginx-controller` service, and copy and paste its `EXTERNAL-IP`. This is the IP address that your application will be running at on the public internet.

>Note: you should go further and set your DNS records appropriately and set up a TLS certificate for this IP address. Instructions to do that are out of scope of this document, though.
