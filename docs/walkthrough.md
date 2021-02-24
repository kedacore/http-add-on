# Getting Started With The HTTP Add On

One of the primary goals of this project is a simple common-case developer experience. After you've installed KEDA and the HTTP Add On (this project), this document will show you how to get started with an example app.

>If you haven't installed KEDA and this project, please do so first. Follow instructions [install.md](./install.md) to complete your installation.

## Submitting an `HTTPScaledObject`

You interact with the operator via a CRD called `HTTPScaledObject`. To get an example app up and running, read the notes below and then run the subsequent command from the root of this repository.

- Make sure that your `NAMESPACE` environment variable is set to the same value as what you [install](./install.md)ed with
  - I recommend using [direnv](https://direnv.net/) to store your environment variables
- This command will install a simple server that is exposed to the internet using a `LoadBalancer` `Service`. _Do not use this for a production deployment_. Support for `Ingress` is forthcoming in [issue #33](https://github.com/kedacore/http-add-on/issues/33)

```shell
kubectl create -f -n $NAMESPACE examples/httpscaledobject.yaml
```
