# The `HTTPScaledObject`

>This document reflects the specification of the `HTTPScaledObject` resource for the latest version

Each `HTTPScaledObject` looks approximately like the below:

```yaml
kind: HTTPScaledObject
apiVersion: http.keda.sh/v1alpha1
metadata:
    name: xkcd
spec:
    scaleTargetRef:
        deployment: xkcd
        service: xkcd
        port: 8080
        targetPendingRequests: 100
```

This document is a narrated reference guide for the `HTTPScaledObject`, and we'll focus on the `spec` field.

## `scaleTargetRef`

This is the primary and most important part of the `spec` because it describes (1) what `Deployment` to scale and (2) where and how to route traffic.

### `deployment`

This is the name of the `Deployment` to scale. It must exist in the same namespace as this `HTTPScaledObject` and shouldn't be managed by any other autoscaling system. This means that there should not be any `ScaledObject` already created for this `Deployment`. The HTTP add on will manage a `ScaledObject` internally.

### `service`

This is the name of the service to route traffic to. The add on will create autoscaling and routing components that route to this `Service`. It must exist in the same namespace as this `HTTPScaledObject` and should route to the same `Deployment` as you entered in the `deployment` field.

### `port`

This is the port to route to on the service that you specified in the `service` field. It should be exposed on the service and should route to a valid `containerPort` on the `Deployment` you gave in the `deployment` field.

### `targetPendingRequests`

>Default: 100

The HTTP Addon works with the Kubernetes [Horizonal Pod Autoscaler](https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/#algorithm-details) (HPA) -- via KEDA itself -- to execute scale-up and scale-down operations (except for scaling between zero and non-zero replicas). This field specifies the total number of pending requests across the interceptor fleet to allow in the interceptors before a scaling operation is triggered.

For example, if you set this field to 100, then the HPA will scale up if it queries the HTTP Addon and finds that there are more than 100 pending requests across the entire interceptor fleet at that moment. Likewise, the HPA will scale down if it finds that there are fewer than 100 pending requests across the fleet.

>Note: this field is used in the (simplified) HPA scaling formula, which is listed [on the Kubernetes documentation](https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/#algorithm-details). It is also pasted here for convenience: `desiredReplicas = ceil[currentReplicas * ( currentMetricValue / desiredMetricValue )]`. The value of `targetPendingRequests` will be passed in where `desiredMetricValue` is expected.
