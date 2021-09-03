# The `HTTPScaledObject`

>This document reflects the specification of the `HTTPScaledObject` resource for the `v0.2.0` version.

Each `HTTPScaledObject` looks approximately like the below:

```yaml
kind: HTTPScaledObject
apiVersion: http.keda.sh/v1alpha1
metadata:
    name: xkcd
spec:
    host: "myhost.com"
    scaleTargetRef:
        deployment: xkcd
        service: xkcd
        port: 8080
```

This document is a narrated reference guide for the `HTTPScaledObject`, and we'll focus on the `spec` field.

## `host`

This is the host to apply this scaling rule to. All incoming requests with this value in their `Host` header will be forwarded to the `Service` and port specified in the below `scaleTargetRef`, and that same `scaleTargetRef`'s `Deployment` will be scaled accordingly.

## `scaleTargetRef`

This is the primary and most important part of the `spec` because it describes:

1. The incoming host to apply this scaling rule to.
2. What `Deployment` to scale.
3. The service to which to route HTTP traffic.

### `deployment`

This is the name of the `Deployment` to scale. It must exist in the same namespace as this `HTTPScaledObject` and shouldn't be managed by any other autoscaling system. This means that there should not be any `ScaledObject` already created for this `Deployment`. The HTTP add on will manage a `ScaledObject` internally.

### `service`

This is the name of the service to route traffic to. The add on will create autoscaling and routing components that route to this `Service`. It must exist in the same namespace as this `HTTPScaledObject` and should route to the same `Deployment` as you entered in the `deployment` field.

### `port`

This is the port to route to on the service that you specified in the `service` field. It should be exposed on the service and should route to a valid `containerPort` on the `Deployment` you gave in the `deployment` field.
