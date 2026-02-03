# The `HTTPScaledObject`

> This document reflects the specification of the `HTTPScaledObject` resource for the `vX.X.X` version.

Each `HTTPScaledObject` looks approximately like the below:

```yaml
kind: HTTPScaledObject
apiVersion: http.keda.sh/v1alpha1
metadata:
  name: xkcd
  annotations:
    httpscaledobject.keda.sh/skip-scaledobject-creation: "false"
spec:
  hosts:
    - myhost.com
    - "*.example.com"
  pathPrefixes:
    - /test
  headers:
    - name: X-Custom-Header
      value: CustomValue
  scaleTargetRef:
    name: xkcd
    kind: Deployment
    apiVersion: apps/v1
    service: xkcd
    port: 8080
  replicas:
    min: 5
    max: 10
  scaledownPeriod: 300
  scalingMetric: # requestRate and concurrency are mutually exclusive
    requestRate:
      granularity: 1s
      targetValue: 100
      window: 1m
    concurrency:
      targetValue: 100
  placeholderConfig:
    enabled: true
    refreshInterval: 5
    statusCode: 503
    headers:
      Content-Type: "text/html; charset=utf-8"
      X-Service-Status: "warming-up"
    content: |
      <!DOCTYPE html>
      <html>
      <head>
          <title>Service Starting</title>
          <meta http-equiv="refresh" content="{{.RefreshInterval}}">
      </head>
      <body>
          <h1>{{.ServiceName}} is starting...</h1>
      </body>
      </html>
```

This document is a narrated reference guide for the `HTTPScaledObject`.

## `httpscaledobject.keda.sh/skip-scaledobject-creation` annotation

This annotation will disable the ScaledObject generation and management but keeping the routing and metrics available. This is done removing the current ScaledObject if it has been already created, allowing to use user managed ScaledObjects pointing the add-on scaler directly (supporting all the ScaledObject configurations and multiple triggers). You can read more about this [here](./../../walkthrough.md#integrating-http-add-on-scaler-with-other-keda-scalers)

## `hosts`

These are the hosts to apply this scaling rule to. All incoming requests with one of these values in their `Host` header will be forwarded to the `Service` and port specified in the below `scaleTargetRef`, and that same `scaleTargetRef`'s workload will be scaled accordingly.

Wildcard patterns are supported using a leading `*.` prefix. For example, `*.example.com` matches any subdomain of `example.com`, including multi-level subdomains like `foo.bar.example.com`. More specific wildcards take precedence over less specific ones (e.g., `*.bar.example.com` wins over `*.example.com`). Exact host matches always take precedence over wildcard matches.

An empty host or `*` acts as a catch-all that matches any hostname. This is useful as a fallback when no other routes match.

## `pathPrefixes`

> Default: "/"

These are the paths to apply this scaling rule to. All incoming requests with one of these values as path prefix will be forwarded to the `Service` and port specified in the below `scaleTargetRef`, and that same `scaleTargetRef`'s workload will be scaled accordingly.

## `headers`

> Default: No headers

To further refine which requests this scaling rule applies to, you can specify a list of HTTP headers. Headers can be specified with or without values - if a value is provided, it must match exactly; if no value is provided, only the header's presence is required. All incoming requests that satisfy these header conditions will be forwarded to the `Service` and port specified in the below `scaleTargetRef`, and that same `scaleTargetRef`'s workload will be scaled accordingly. Most specific matches take precedence over less specific ones. This means that rules with more headers defined will be prioritized over those with fewer headers when multiple rules could apply to a request. Also a match for header with key and value takes precedence over a match for header with only a key.

## `scaleTargetRef`

This is the primary and most important part of the `spec` because it describes:

1. The incoming host to apply this scaling rule to.
2. What workload to scale.
3. The service to which to route HTTP traffic.

### `name`

This is the name of the workload to scale. It must exist in the same namespace as this `HTTPScaledObject` and shouldn't be managed by any other autoscaling system. This means that there should not be any `ScaledObject` already created for this workload. The HTTP Add-on will manage a `ScaledObject` internally.

### `kind`

This is the kind of the workload to scale.

### `apiVersion`

This is the apiVersion of the workload to scale.

### `service`

This is the name of the service to route traffic to. The add-on will create autoscaling and routing components that route to this `Service`. It must exist in the same namespace as this `HTTPScaledObject` and should route to the same `Deployment` as you entered in the `deployment` field.

### `port`

This is the port to route to on the service that you specified in the `service` field. It should be exposed on the service and should route to a valid `containerPort` on the workload you gave.

### `portName`

Alternatively, the port can be referenced using it's `name` as defined in the `Service`.

### `scaledownPeriod`

> Default: 300

The period to wait after the last reported active before scaling the resource back to 0.

> Note: This time is measured on KEDA side based on in-flight requests, so workloads with few and random traffic could have unexpected scale to 0 cases. In those case we recommend to extend this period to ensure it doesn't happen.

## `scalingMetric`

This is the second most important part of the `spec` because it describes how the workload has to scale. This section contains 2 nested sections (`requestRate` and `concurrency`) which are mutually exclusive between themselves.

### `requestRate`

This section enables scaling based on the request rate.

> **NOTE**: Requests information is stored in memory, aggragating long periods (longer than 5 minutes) or too fine granularity (less than 1 second) could produce perfomance issues or memory usage increase.

> **NOTE 2**: Although updating `window` and/or `granularity` is something doable, the process just replaces all the stored request count infomation. This can produce unexpected scaling behaviours until the window is populated again.

#### `targetValue`

> Default: 100

This is the target value for the scaling configuration.

#### `window`

> Default: "1m"

This value defines the aggregation window for the request rate calculation.

#### `granularity`

> Default: "1s"

This value defines the granualarity of the aggregated requests for the request rate calculation.

### `concurrency`

This section enables scaling based on the request concurrency.

> **NOTE**: This is the only scaling behaviour before v0.8.0

#### `targetValue`

> Default: 100

This is the target value for the scaling configuration.

## `placeholderConfig`

This optional section enables serving placeholder responses when the workload is scaled to zero. When enabled, instead of returning an error while waiting for the workload to scale up, the interceptor will serve a customizable response with any content format.

### `enabled`

>Default: false

Whether to enable placeholder responses for this HTTPScaledObject.

### `refreshInterval`

>Default: 5

A template variable (in seconds) that can be used in your content template. This is just data passed to the template - it does not automatically refresh the response. You can use it in your content for client-side refresh logic if needed (e.g., `<meta http-equiv="refresh" content="{{.RefreshInterval}}">`).

### `statusCode`

>Default: 503

The HTTP status code to return with the placeholder response. Common values are 503 (Service Unavailable) or 202 (Accepted).

### `headers`

>Default: {}

A map of custom HTTP headers to include in the placeholder response. **Important**: Use this to set the `Content-Type` header to match your content format. For example:
- `Content-Type: text/html; charset=utf-8` for HTML
- `Content-Type: application/json` for JSON
- `Content-Type: text/plain` for plain text

### `content`

>Default: ConfigMap-provided template (if configured), otherwise returns simple text

Custom content for the placeholder response. Supports any format (HTML, JSON, XML, plain text, etc.). Content is processed as a Go template with the following variables:
- `{{.ServiceName}}` - The name of the service from scaleTargetRef
- `{{.Namespace}}` - The namespace of the HTTPScaledObject
- `{{.RefreshInterval}}` - The configured refresh interval value (just a number)
- `{{.RequestID}}` - The X-Request-ID header value if present
- `{{.Timestamp}}` - The current timestamp in RFC3339 format

**Examples:**

HTML with client-side refresh:
```yaml
content: |
  <!DOCTYPE html>
  <html>
  <head>
    <title>Service Starting</title>
    <meta http-equiv="refresh" content="{{.RefreshInterval}}">
  </head>
  <body>
    <h1>{{.ServiceName}} is starting...</h1>
  </body>
  </html>
headers:
  Content-Type: "text/html; charset=utf-8"
```

JSON response:
```yaml
content: |
  {
    "status": "warming_up",
    "service": "{{.ServiceName}}",
    "namespace": "{{.Namespace}}",
    "timestamp": "{{.Timestamp}}"
  }
headers:
  Content-Type: "application/json"
```

Plain text:
```yaml
content: "{{.ServiceName}} is starting up. Please retry in a few seconds."
headers:
  Content-Type: "text/plain"
```
