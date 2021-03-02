# The Scope of the KEDA HTTP Add On

Running production HTTP servers in Kubernetes is complicated and involves many pieces of infrastructure. The HTTP Add On aims to autoscale these HTTP servers, but does not aim to extend beyond that scope.

## Autoscaling HTTP

To autoscale HTTP servers, the HTTP Add On needs access to metrics that it can report to KEDA, so that KEDA itself can scale the target HTTP server. The mechanism by which the add on does this is to use an [interceptor](../interceptor) and [external scaler](../scaler). An operator watches for a `HTTPScaledObject` resource and creates these components as necessary.

The HTTP Add On only includes the necessary infrastructure to respond to new, modified, or deleted `HTTPScaledObject`s, and when one is created, the add on only creates the infrastructure needed specifically to accomplish autoscaling.

>As stated above, the current architecture requires an interceptor, which needs to proxy incoming HTTP requests in order to provide autoscaling metrics. That means the scope of the HTTP add on currently needs to include the app's network traffic routing system
