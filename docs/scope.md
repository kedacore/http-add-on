# Why build an HTTP add-on?

Running production HTTP servers in Kubernetes is complicated and involves many pieces of infrastructure. The HTTP Add-on (called the "add-on" hereafter) aims to autoscale these HTTP servers, but does not aim to extend beyond that scope. Generally, this project only aims to do two things:

1. Autoscale arbitrary HTTP servers based on the volume of traffic incoming to it, including to zero.
2. Route HTTP traffic from a given source to an arbitrary HTTP server, as far as we need to efficiently accomplish (1).

The add-on only provides this functionality to workloads that _opt in_ to it. We provide more detail below.

### Autoscaling HTTP

To autoscale HTTP servers, the HTTP Add-on needs access to metrics that it can report to KEDA, so that KEDA itself can scale the target HTTP server. The mechanism by which the add-on does this is to use an [interceptor](../interceptor) and [external scaler](../scaler). An operator watches for a `HTTPScaledObject` resource and creates these components as necessary.

The HTTP Add-on only includes the necessary infrastructure to respond to new, modified, or deleted `HTTPScaledObject`s, and when one is created, the add-on only creates the infrastructure needed specifically to accomplish autoscaling.

>As stated above, the current architecture requires an "interceptor", which needs to proxy incoming HTTP requests in order to provide autoscaling metrics. That means the scope of the HTTP Add-on currently needs to include the app's network traffic routing system.

To learn more, we recommend reading about our [design](design.md) or go through our [FAQ](faq.md).