# Integrations

## Istio

### Configuration Steps

1. **Proxy Service in Virtual Service:**

   - Within the Istio virtual service definition, add a proxy service as a route destination.
   - Set the host of this proxy service to `keda-add-ons-http-interceptor-proxy`` (the KEDA HTTP Addon interceptor service).
   - Set the port to `8080`` (the default interceptor port).

**Example yaml**

```yaml
apiVersion: networking.istio.io/v1alpha3
kind: VirtualService
metadata:
  name: example
  namespace: default
spec:
  http:
    - route:
        - destination:
            host: keda-add-ons-http-interceptor-proxy
            port: 8080
```

2. **Namespace Alignment:**

   - Ensure that both the KEDA HTTP Addon and the Istio virtual service are deployed within the same Kubernetes namespace. This ensures proper communication between the components.

### Behavior

- When a user makes a request, the Istio virtual service routes it to the KEDA HTTP Addon interceptor service.
- The interceptor service captures request metrics and relays them to the KEDA scaler component.
- Based on these metrics and scaling rules defined in the KEDA configuration, the KEDA scaler automatically scales the target workload (e.g., a deployment) up or down (including scaling to zero).

### Troubleshooting Tips

1. **Error: `context marked done while waiting for workload reach > 0 replicas`**

   - This error indicates that the `KEDA_CONDITION_WAIT_TIMEOUT` value (default: 20 seconds) might be too low. The workload scaling process may not be complete within this timeframe.
   - To increase the timeout:
     - If using Helm, adjust the `interceptor.replicas.waitTimeout`` parameter (see reference below).
     - Reference: [https://github.com/kedacore/charts/blob/main/http-add-on/values.yaml#L139](https://github.com/kedacore/charts/blob/main/http-add-on/values.yaml#L139)

2. **502 Errors with POST Requests:**

   - You might encounter 502 errors during POST requests when the request is routed through the interceptor service. This could be due to insufficient timeout settings.
   - To adjust timeout parameters:
     - If using Helm, modify the following parameters (see reference below):
       - `KEDA_HTTP_CONNECT_TIMEOUT`
       - `KEDA_RESPONSE_HEADER_TIMEOUT`
       - `KEDA_HTTP_EXPECT_CONTINUE_TIMEOUT`
     - Reference: [https://github.com/kedacore/charts/blob/main/http-add-on/values.yaml#L152](https://github.com/kedacore/charts/blob/main/http-add-on/values.yaml#L152)

3. **Immediate Scaling Down to Zero:**
   - If `minReplica` is set to 0 in the HTTPScaledObject, the application will immediately scale down to 0.
   - There's currently no built-in mechanism to delay this initial scaling.
   - A PR is in progress to add this support: [https://github.com/kedacore/keda/pull/5478](https://github.com/kedacore/keda/pull/5478)
   - As a workaround, keep `minReplica` initially as 1 and update it to 0 after the desired delay.
