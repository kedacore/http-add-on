# NetworkPolicy Examples

Example [NetworkPolicy](https://kubernetes.io/docs/concepts/services-networking/network-policies/) manifests for the KEDA HTTP Add-on components and the [`examples/xkcd`](../xkcd) sample application.

These are starting points for clusters with a CNI that enforces NetworkPolicy (for example Calico or Cilium). Adjust namespace names, label selectors, and `ipBlock` rules to match your environment before applying them in production.

Add-on selectors use `app.kubernetes.io/name: http-add-on` and `app.kubernetes.io/component`. `component: operator` alone also matches KEDA's own operator, metrics API server, and admission webhook.

## Files

| File | Target |
| --- | --- |
| `interceptor-networkpolicy.yaml` | Interceptor proxy and admin API |
| `operator-networkpolicy.yaml` | HTTP Add-on operator |
| `scaler-networkpolicy.yaml` | External scaler |
| `app-networkpolicy.yaml` | Sample application (`examples/xkcd`) |

## Apply

```bash
kubectl apply -f examples/networkpolicy/interceptor-networkpolicy.yaml
kubectl apply -f examples/networkpolicy/operator-networkpolicy.yaml
kubectl apply -f examples/networkpolicy/scaler-networkpolicy.yaml
kubectl apply -f examples/networkpolicy/app-networkpolicy.yaml
```

The add-on policies assume the HTTP Add-on and KEDA are installed in the `keda` namespace. The application policy assumes the xkcd example runs in `default`.

## Customize

- **Namespace**: Change `metadata.namespace` if your add-on or application runs elsewhere.
- **Ingress controller**: The interceptor ingress rule matches ingress-nginx. Replace the namespace and pod labels for another controller. Traffic from one application to another through the interceptor is not allowed by this rule.
- **Backend egress**: The interceptor policy allows egress to xkcd pods in `default` on port 8080. Point `to` at your application namespace, labels, and port.
- **Health probes**: Kubernetes allows connections from a pod's own node when ingress is restricted, so kubelet probes do not need an extra rule.
- **Kubernetes API**: The API server is usually a ClusterIP or control-plane IP, not a selectable pod. The example allows ports 443 and 6443 to any destination; restrict that with `ipBlock` in production.
- **Metrics**: These examples do not allow Prometheus scrapes. Add an ingress rule for your collector if you need one. The interceptor and scaler expose Prometheus on port `2223`. Helm serves operator metrics on `8443` (HTTPS) by default. Remote OpenTelemetry export (usually `4317` and `4318`) is blocked; add egress to your collector if you enable it.
- **External egress**: Use `ipBlock` for traffic outside the cluster. `namespaceSelector` and `podSelector` only match in-cluster endpoints.

## Verify

The xkcd chart defaults to `minReplicas: 0`. Keep one replica so a failed request is the policy, not scale-to-zero:

```bash
helm upgrade --install xkcd ./examples/xkcd --namespace default --set autoscaling.http.minReplicas=1
kubectl -n default wait --for=jsonpath='{.subsets[0].addresses[0].ip}' endpoints/xkcd
```

Install the HTTP Add-on, apply the policies, and confirm requests through your ingress controller still succeed. From a pod that is not the interceptor, curling the xkcd Service should time out:

```bash
kubectl run curl --rm -it --restart=Never --image=curlimages/curl -- \
  curl -sS --max-time 5 http://xkcd.default.svc:8080/
```

Delete `app-networkpolicy.yaml` and run the same curl again. It should succeed while the endpoint is still ready.
