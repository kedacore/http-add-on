# The `HTTPScaledObject`

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
```

This document is a narrated reference guide for the `HTTPScaledObject`, and we'll focus on the `spec` field.

## `scaleTargetRef`

This is the primary and most important part of the `spec` because it describes (1) what `Deployment` to scale and (2) where and how to route traffic.

### `deployment`

This is the name of the `Deployment` to scale. It must exist in the same namespace as this `HTTPScaledObject` and shouldn't be managed by any other autoscaling system. This means that there should not be any `ScaledObject` already created for this `Deployment`. The HTTP add on will manage a `ScaledObject` internally.

## Replicas

Describes the minimum and maximum amount of replicas to have in the scaled app.

### min

Minimum amount of replicas.

### max

Maximum amount of replicas.

## Service

Describes the interceptor service that will be created.

### `type`

Type of the exposed service created to serve de interceptor. Can be one of `LoadBalancer`, `ClusterIP`, or `NodePort`.

### `port`

This is the port to route to on the service that you specified in the `service` field. It should be exposed on the service and should route to a valid `containerPort` on the `Deployment` you gave in the `deployment` field.

### `name`

This is the name of the service to route traffic to. The add on will create autoscaling and routing components that route to this `Service`. It must exist in the same namespace as this `HTTPScaledObject` and should route to the same `Deployment` as you entered in the `deployment` field.
