# Getting Started With The HTTP Add-on

After you've installed KEDA and the HTTP Add-on (this project, we'll call it the "add-on" for short), this document will show you how to get started with an example app.

If you haven't installed KEDA and the HTTP Add-on (this project), please do so first. Follow instructions [install.md](./install.md) to complete your installation.

>Before you continue, make sure that you have your `NAMESPACE` environment variable set to the same value as it was when you installed.

## Creating An Application

You'll need to install a `Deployment` and `Service` first. You'll tell the add-on to begin scaling it up and down after this step. We've provided a [Helm](https://helm.sh) chart in this repository that you can use to try it out. Use this command to create the resources you need.

```console
helm install xkcd ./examples/xkcd -n ${NAMESPACE}
```

#### xkcd exposed with GatewayAPI
Alternatively if you'd like to try the addon along with GatewayAPI, you can install first GatewayAPI CRDs and some GatewayAPI implementation, for example as described in a [section below](#installing-and-using-the-eg-gatewayapi) and install the application as with `httproute=true` which will deploy properly configured `HTTPRoute` too.

```console
helm install xkcd ./examples/xkcd -n ${NAMESPACE} --set httproute=true
```

You'll need to clone the repository to get access to this chart. If you have your own workload and `Service` installed, you can go right to creating an `HTTPScaledObject` in the next section.

>If you are running KEDA and the HTTP Add-on in cluster-global mode, you can install the XKCD chart in any namespace you choose. If you do so, make sure you add `--set ingressNamespace=${NAMESPACE}` to the above installation command.

>To remove the app, run `helm delete xkcd -n ${NAMESPACE}`

## Creating an `HTTPScaledObject`

You interact with the operator via a CRD called `HTTPScaledObject`. This CRD object instructs interceptors to forward requests for a given host to your app's backing `Service`. To get an example app up and running, read the notes below and then run the subsequent command from the root of this repository.

```console
kubectl apply -n $NAMESPACE -f examples/v0.8.0/httpscaledobject.yaml
```

>If you'd like to learn more about this object, please see the [`HTTPScaledObject` reference](./ref/v0.8.0/http_scaled_object.md).

## Testing Your Installation

You've now installed a web application and activated autoscaling by creating an `HTTPScaledObject` for it. For autoscaling to work properly, HTTP traffic needs to route through the `Service` that the add-on has set up. You can use `kubectl port-forward` to quickly test things out:

```console
kubectl port-forward svc/keda-http-add-on-interceptor-proxy -n ${NAMESPACE} 8080:8080
```

### Routing to the Right `Service`

As said above, you need to route your HTTP traffic to the `Service` that the add-on has created during the installation. If you have existing systems - like an ingress controller - you'll need to anticipate the name of these created `Service`s. Each one will be named consistently like so, in the same namespace as the `HTTPScaledObject` and your application (i.e. `$NAMESPACE`):

```console
keda-http-add-on-interceptor-proxy
```

> This is installed by raw manifests. If you are using the [Helm chart](https://github.com/kedacore/charts/tree/main/http-add-on) to install the add-on, it crates a service named `keda-add-ons-http-interceptor-proxy` as a `ClusterIP` by default.

#### Installing and Using the [ingress-nginx](https://kubernetes.github.io/ingress-nginx/deploy/#using-helm) Ingress Controller

As mentioned above, the `Service` that the add-on creates will be inaccessible over the network from outside of your Kubernetes cluster.

While you can access it via the `kubectl port-forward` command above, we recommend against using that in a production setting. Instead, we recommend that you use an [ingress controller](https://kubernetes.io/docs/concepts/services-networking/ingress-controllers/) to route to the interceptor service. This section describes how to set up and use the NGINX Ingress controller.

First, install the controller using the commands below. These commands use Helm v3. For other installation methods, see the [installation page](https://kubernetes.github.io/ingress-nginx/deploy/).

```console
helm repo add ingress-nginx https://kubernetes.github.io/ingress-nginx
helm repo update
helm install ingress-nginx ingress-nginx/ingress-nginx -n ${NAMESPACE}
```

An [`Ingress`](https://kubernetes.io/docs/concepts/services-networking/ingress/) resource was already created as part of the [xkcd chart](../examples/xkcd/templates/ingress.yaml), so the installed NGINX ingress controller will initialize, detect the `Ingress`, and begin routing to the xkcd interceptor `Service`.

>NOTE: You may have to create an external service `type: ExternalName` pointing to the interceptor namespace and use it from `Ingress` manifest.

When you're ready, please run `kubectl get svc -n ${NAMESPACE}`, find the `ingress-nginx-controller` service, and copy and paste its `EXTERNAL-IP`. This is the IP address that your application will be running at on the public internet.

>Note: you should go further and set your DNS records appropriately and set up a TLS certificate for this IP address. Instructions to do that are out of scope of this document, though.

#### Installing and Using the [eg](https://gateway.envoyproxy.io/latest/install/install-helm/) GatewayAPI

Similarly to exposing your service with `Ingress`, you can expose your service with `HTTPRoute` as part of [GatewayAPI](https://github.com/kubernetes-sigs/gateway-api). Following steps describe how to install one of may GatewayAPI implementations - Envoy Gateway.
You should install the `xkcd` helm chart with `--set httproute=true` as [explained above](#xkcd-exposed-with-gatewayapi).

The helm chart is publically available and hosted by DockerHub
```console
helm install eg oci://docker.io/envoyproxy/gateway-helm --version v1.0.1 -n envoy-gateway-system --create-namespace
```
Before creating new `Gateway`, wait for Envoy Gateway to become available
```console
kubectl wait --timeout=5m -n envoy-gateway-system deployment/envoy-gateway --for=condition=Available
```
Create `GatewayClass` and `Gateway`
```console
cat << 'EOF' | kubectl apply -f -
apiVersion: gateway.networking.k8s.io/v1
kind: GatewayClass
metadata:
  name: eg
spec:
  controllerName: gateway.envoyproxy.io/gatewayclass-controller
---
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: eg
  namespace: envoy-gateway-system
spec:
  gatewayClassName: eg
  listeners:
    - name: http
      protocol: HTTP
      port: 80
      allowedRoutes:
        namespaces:
          from: All
EOF
```
> 💡 Note the `ExternalName` type `Service` used to route traffic from `Ingress` defined in one namespace to the interceptor `Service` defined in another is not necessary with GatewayAPI.
> The GatewayAPI defines [`ReferenceGrant`](https://gateway-api.sigs.k8s.io/api-types/referencegrant/) to allow `HTTPRoutes` referencing `Services` and other types of backend from different `Namespaces`.

You can see the IP address for following rest of the document with
```console
kubectl get gateway -n envoy-gateway-system
```
For example (your IP will likely differ)
```
NAME   CLASS   ADDRESS          PROGRAMMED   AGE
eg     eg      172.24.255.201   True         16s
```

### Making an HTTP Request to your App

Now that you have your application running and your ingress configured, you can issue an HTTP request. To do so, you'll need to know the IP address to request. If you're using an ingress controller, that is the IP of the ingress controller's `Service`. If you're using a "raw" `Service` with `type: LoadBalancer`, that is the IP address of the `Service` itself.

Regardless, you can use the below `curl` command to make a request to your application:

```console
curl -H "Host: myhost.com" <Your IP>/path1
```

>Note the `-H` flag above to specify the `Host` header. This is needed to tell the interceptor how to route the request. If you have a DNS name set up for the IP, you don't need this header.

You can also use port-forward to interceptor service for making the request:

```console
kubectl port-forward svc/keda-http-add-on-interceptor-proxy -n ${NAMESPACE} 8080:8080
curl -H "Host: myhost.com" localhost:8080/path1
```

### Integrating HTTP Add-On Scaler with other KEDA scalers

For scenerios where you want to integrate HTTP Add-On scaler with other keda scalers, you can set the `"httpscaledobject.keda.sh/skip-scaledobject-creation"` annotation to true on your `HTTPScaledObject`.  The reconciler will then skip the KEDA core ScaledObject creation which will allow you to create your own `ScaledObject` and add HTTP scaler as one of your triggers.

> 💡 Ensure that your ScaledObject is created with a different name than the `HTTPScaledObject` to ensure your ScaledObject is not removed by the reconciler.

If you don't know how to set the external scaler in the ScaledObject, you can deploy first your HTTPScaledObject with no annotation set in order to obtain the latest trigger spec to use on your own managed ScaledObject.

1. Deploy your `HTTPScaledObject` with annotation set to false

```console
annotations:
  httpscaledobject.keda.sh/skip-scaledobject-creation: "false"
```

2. Take copy of the current generated external-push trigger spec on the generated ScaledObject.

For example:

```console
  triggers:
  - type: external-push
    metadata:
      httpScaledObject: YOUR_HTTPSCALEDOBJECT_NAME
      scalerAddress: keda-http-add-on-external-scaler.keda:9090
```

3. Apply the `"httpscaledobject.keda.sh/skip-scaledobject-creation"` annotation with `true` and apply the change. This will remove the originally created `ScaledObject` allowing you to create your own.

```console
annotations:
  httpscaledobject.keda.sh/skip-scaledobject-creation: "true"
```

4. Add the `external-push` trigger taken from step 2 to your own ScaledObject and apply this.


[Go back to landing page](./)
