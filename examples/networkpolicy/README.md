# NetworkPolicy Examples

Example [NetworkPolicy](https://kubernetes.io/docs/concepts/services-networking/network-policies/) manifests for the KEDA HTTP Add-on components and the [`examples/xkcd`](../xkcd) sample application.

These are starting points for clusters with a CNI that enforces NetworkPolicy (for example Calico or Cilium). Adjust namespace names, label selectors, and `ipBlock` rules to match your environment before applying them in production.

Selectors match the Helm chart labels (`app.kubernetes.io/component`).

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
- **Ingress controller**: Update the ingress rule in `interceptor-networkpolicy.yaml` to match your controller's namespace and pod labels.
- **Backend egress**: The interceptor policy allows egress to pods in `default` on port 8080. Point `to` at your application namespaces and ports.
- **Health probes**: Kubelet probes come from node IPs, not pods. If probes fail after applying a policy, add an ingress `ipBlock` for your node CIDR.
- **Kubernetes API**: The API server is usually a ClusterIP or control-plane IP, not a selectable pod. The example allows ports 443 and 6443 to any destination; restrict that with `ipBlock` in production.
- **Metrics**: These examples do not allow Prometheus scrape. If you need it, add an ingress rule for your collector. Helm serves operator metrics on `8443` (HTTPS) by default, and the scaler Prometheus exporter on `2223`.
- **External egress**: Use `ipBlock` for traffic outside the cluster. `namespaceSelector` and `podSelector` only match in-cluster endpoints.

## Verify

Install the HTTP Add-on and the [xkcd example](../xkcd), then apply the policies.

Requests that reach the interceptor through your ingress controller should still succeed. From a pod that is not the interceptor, curling the xkcd Service should time out:

```bash
kubectl run curl --rm -it --restart=Never --image=curlimages/curl -- \
  curl -sS --max-time 5 http://xkcd.default.svc:8080/
```

`kubectl port-forward` to the interceptor is not a valid check: the interceptor policy only allows proxy traffic from the ingress controller you configured.
