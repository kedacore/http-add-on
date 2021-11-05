# KEDA-HTTP Frequently Asked Questions

## Why Does This Project Route HTTP Requests?

In order to autoscale a `Deployment`, KEDA-HTTP needs to be involved with routing HTTP requests. However, the project is minimally involved with routing and we're working on ways to get out of the "critical path" of an HTTP request as much as possible. For more information, please see our [scope](./scope.md) document.

## How is this Project Similar or Different from [Osiris](https://github.com/deislabs/osiris)?

Osiris and KEDA-HTTP have similar features:

- Autoscaling, including scale-to-zero of compute workloads
- Native integration to the [Horizontal Pod Autoscaler](https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/)

However, Osiris and KEDA-HTTP differ in several ways:

- Autoscaling concerns are implemented separately from the application resources - `Service`, `Ingress`, `Deployment` and more in KEDA-HTTP. With Osiris, those concerns are baked into each app resource.
- The KEDA-HTTP operator can automatically deploy and configure networking and compute resources necessary for an HTTP application to autoscale. Osiris does not have this functionality.
- Osiris is currently archived in GitHub

## How is this Project Similar or Different from [Knative](https://knative.dev/)?

Knative Serving and KEDA-HTTP both have core support for autoscaling, including scale-to-zero of compute workloads. KEDA-HTTP is focused solely on deploying production-grade autoscaling HTTP applications, while Knative builds in additional functionality:

- Pure [event-based workloads](https://knative.dev/docs/eventing/). [KEDA core](https://github.com/kedacore/keda), without KEDA-HTTP, can support such workloads natively.
- Complex deployment strategies like [blue-green](https://knative.dev/docs/serving/samples/blue-green-deployment/)
- Supporting other autoscaling mechanisms beyond the built-in [HPA](https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/), such as the [Knative Pod Autoscaler (KPA)](https://knative.dev/docs/serving/autoscaling/autoscaling-concepts/#knative-pod-autoscaler-kpa)

Additionally, Knative supports a service mesh, while KEDA-HTTP does not out of the box (support for that is forthcoming).

## How is this Project Similar or Different from [OpenFaaS](https://www.openfaas.com/)

OpenFaaS and KEDA-HTTP both can deploy and autoscale HTTP workloads onto Kubernetes, but they have several important differences that make them suitable for different use cases:

- OpenFaaS requires the use of a CLI to deploy code to production
- OpenFaaS primarily supports the event-based "functions as a service" pattern. This means:
  - You deploy code, in a supported language, to handle an HTTP request and OpenFaaS takes care of serving and invoking your code for you
  - You deploy complete containers with your HTTP server process in them to KEDA-HTTP
- You don't need to build a container image to deploy code to OpenFaaS, while you do to deploy to KEDA-HTTP
- OpenFaaS can run either on or off Kubernetes, while KEDA-HTTP is Kubernetes-only
- OpenFaaS requires several additional components when running in Kubernetes, like Prometheus. The documentation refers to this as the [PLONK stack](https://docs.openfaas.com/deployment/#plonk-stack)
