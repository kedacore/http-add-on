# KEDA HTTP Add-On

Here is an overview of detailed documentation:

- [Install](install.md)
- [Design](design.md)
- [Use-Cases](use_cases.md)
- [Walkthrough](walkthrough.md)
- [Developing](developing.md)

## Why build an HTTP add-on?

Running production HTTP servers in Kubernetes is complicated and involves many pieces of infrastructure. The HTTP Add-on (called the "add-on" hereafter) aims to autoscale these HTTP servers, but does not aim to extend beyond that scope. Generally, this project only aims to do two things:

1. Autoscale arbitrary HTTP servers based on the volume of traffic incoming to it, including to zero.
2. Route HTTP traffic from a given source to an arbitrary HTTP server, as far as we need to efficiently accomplish (1).

The add-on only provides this functionality to workloads that _opt in_ to it. We provide more detail below.

### Autoscaling HTTP

To autoscale HTTP servers, the HTTP Add-on needs access to metrics that it can report to KEDA, so that KEDA itself can scale the target HTTP server. The mechanism by which the add-on does this is to use an [interceptor](../interceptor) and [external scaler](../scaler). An operator watches for a `HTTPScaledObject` resource and creates these components as necessary.

The HTTP Add-on only includes the necessary infrastructure to respond to new, modified, or deleted `HTTPScaledObject`s, and when one is created, the add-on only creates the infrastructure needed specifically to accomplish autoscaling.

>As stated above, the current architecture requires an "interceptor", which needs to proxy incoming HTTP requests in order to provide autoscaling metrics. That means the scope of the HTTP Add-on currently needs to include the app's network traffic routing system.

To learn more, we recommend reading about our [design](design.md) or go through our [FAQ](faq.md).


## FAQ

## Why does this project route HTTP requests?

In order to autoscale a `Deployment`, KEDA-HTTP needs to be involved with routing HTTP requests. However, the project is minimally involved with routing and we're working on ways to get out of the "critical path" of an HTTP request as much as possible. For more information, please see our [scope](./scope.md) document.

## How is this project similar or different from [Osiris](https://github.com/deislabs/osiris)?

Osiris and KEDA-HTTP have similar features:

- Autoscaling, including scale-to-zero of compute workloads
- Native integration to the [Horizontal Pod Autoscaler](https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/)

However, Osiris and KEDA-HTTP differ in several ways:

- Autoscaling concerns are implemented separately from the application resources - `Service`, `Ingress`, `Deployment` and more in KEDA-HTTP. With Osiris, those concerns are baked into each app resource.
- The KEDA-HTTP operator can automatically deploy and configure networking and compute resources necessary for an HTTP application to autoscale. Osiris does not have this functionality.
- Osiris is currently archived in GitHub.

## How is this project similar or different from [Knative](https://knative.dev/)?

Knative Serving and KEDA-HTTP both have core support for autoscaling, including scale-to-zero of compute workloads. KEDA-HTTP is focused solely on deploying production-grade autoscaling HTTP applications, while Knative builds in additional functionality:

- Pure [event-based workloads](https://knative.dev/docs/eventing/). [KEDA core](https://github.com/kedacore/keda), without KEDA-HTTP, can support such workloads natively.
- Complex deployment strategies like [blue-green](https://knative.dev/docs/serving/samples/blue-green-deployment/).
- Supporting other autoscaling mechanisms beyond the built-in [HPA](https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/), such as the [Knative Pod Autoscaler (KPA)](https://knative.dev/docs/serving/autoscaling/autoscaling-concepts/#knative-pod-autoscaler-kpa).

Additionally, Knative supports a service mesh, while KEDA-HTTP does not out of the box (support for that is forthcoming).

## How is this project similar or different from [OpenFaaS](https://www.openfaas.com/)

OpenFaaS and KEDA-HTTP both can deploy and autoscale HTTP workloads onto Kubernetes, but they have several important differences that make them suitable for different use cases:

- OpenFaaS requires the use of a CLI to deploy code to production.
- OpenFaaS primarily supports the event-based "functions as a service" pattern. This means:
  - You deploy code, in a supported language, to handle an HTTP request and OpenFaaS takes care of serving and invoking your code for you.
  - You deploy complete containers with your HTTP server process in them to KEDA-HTTP.
- You don't need to build a container image to deploy code to OpenFaaS, while you do to deploy to KEDA-HTTP.
- OpenFaaS can run either on or off Kubernetes, while KEDA-HTTP is Kubernetes-only.
- OpenFaaS requires several additional components when running in Kubernetes, like Prometheus. The documentation refers to this as the [PLONK stack](https://docs.openfaas.com/deployment/#plonk-stack).
