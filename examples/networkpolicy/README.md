# NetworkPolicy Examples for KEDA HTTP Add-on.

This directory contains NetworkPolicy examples for securing the KEDA HTTP Add-on components in production environments following the principle of least privilege.

## Overview

The KEDA HTTP Add-on consists of three main components:
- **Interceptor**: HTTP proxy that intercepts and queues requests
- **Operator**: Kubernetes operator managing HTTPScaledObject CRDs
- **Scaler**: External scaler providing metrics to KEDA

These NetworkPolicies restrict network traffic to only what's necessary for each component to function.

## Prerequisites

- Kubernetes cluster with NetworkPolicy support (requires a CNI plugin like Calico, Cilium, or Weave)
- KEDA HTTP Add-on installed in the `keda` namespace
- Understanding of Kubernetes NetworkPolicy concepts

## Quick Start

### Apply All Policies

```bash
kubectl apply -f examples/networkpolicy/
```

### Apply Individual Policies

```bash
# For HTTP Add-on components
kubectl apply -f examples/networkpolicy/interceptor-networkpolicy.yaml
kubectl apply -f examples/networkpolicy/operator-networkpolicy.yaml
kubectl apply -f examples/networkpolicy/scaler-networkpolicy.yaml

# For your application (example)
kubectl apply -f examples/networkpolicy/app-networkpolicy.yaml
```

## NetworkPolicy Files

### 1. `interceptor-networkpolicy.yaml`
Secures the Interceptor component:
- **Ingress**: Allows HTTP/HTTPS traffic from ingress controllers, scaler admin API access, and metrics scraping
- **Egress**: Allows connections to backend services, Kubernetes API, and DNS

### 2. `operator-networkpolicy.yaml`
Secures the Operator component:
- **Ingress**: Allows metrics scraping and health probes
- **Egress**: Allows connections to Kubernetes API and DNS

### 3. `scaler-networkpolicy.yaml`
Secures the Scaler component:
- **Ingress**: Allows gRPC connections from KEDA and health probes
- **Egress**: Allows connections to interceptor admin API, Kubernetes API, and DNS

### 4. `app-networkpolicy.yaml`
Example NetworkPolicy for user applications:
- **Ingress**: Allows HTTP traffic from interceptor
- **Egress**: Allows connections to external services and DNS

## Customization

### Namespace

These policies assume the HTTP Add-on is installed in the `keda` namespace. If using a different namespace, update the `namespace` field in each policy.

### Ingress Controllers

The interceptor policy allows traffic from pods with label `app.kubernetes.io/name: ingress-nginx`. Update the `namespaceSelector` and `podSelector` to match your ingress controller:

```yaml
# For Traefik
- from:
  - namespaceSelector:
      matchLabels:
        name: traefik
    podSelector:
      matchLabels:
        app.kubernetes.io/name: traefik
```

### Monitoring

If using Prometheus, ensure the monitoring namespace and pod labels match:

```yaml
# Example for Prometheus in monitoring namespace
- from:
  - namespaceSelector:
      matchLabels:
        name: monitoring
    podSelector:
      matchLabels:
        app.kubernetes.io/name: prometheus
```

### KEDA Namespace

The scaler policy allows ingress from KEDA operator. Update if KEDA is in a different namespace:

```yaml
- from:
  - namespaceSelector:
      matchLabels:
        name: keda  # Change to your KEDA namespace
    podSelector:
      matchLabels:
        app: keda-operator
```

## Testing

### Verify Policies Are Applied

```bash
kubectl get networkpolicies -n keda
```

### Test Connectivity

1. **Test Interceptor can reach backend**:
```bash
# Deploy test application
kubectl apply -f examples/networkpolicy/app-networkpolicy.yaml

# Send request through interceptor
curl http://<interceptor-service>/
```

2. **Test Scaler can reach Interceptor**:
```bash
# Check scaler logs for successful queue count fetches
kubectl logs -n keda deployment/keda-add-ons-http-external-scaler
```

3. **Test KEDA can reach Scaler**:
```bash
# Check KEDA operator logs
kubectl logs -n keda deployment/keda-operator
```

### Troubleshooting

If connections fail after applying policies:

1. **Check policy is applied**:
```bash
kubectl describe networkpolicy <policy-name> -n keda
```

2. **Verify pod labels match selectors**:
```bash
kubectl get pods -n keda --show-labels
```

3. **Test with policy temporarily removed**:
```bash
kubectl delete networkpolicy <policy-name> -n keda
# Test connectivity
# Reapply policy
kubectl apply -f examples/networkpolicy/<policy-file>.yaml
```

4. **Check CNI plugin logs** for NetworkPolicy enforcement issues

## Security Considerations

### Principle of Least Privilege

These policies follow the principle of least privilege:
- Only necessary ports are exposed
- Only authorized sources can connect
- Egress is restricted to required destinations

### Defense in Depth

NetworkPolicies are one layer of security. Also consider:
- **RBAC**: Restrict API access (already configured in HTTP Add-on)
- **Pod Security Standards**: Enforce security contexts
- **Service Mesh**: Add mTLS for inter-service communication
- **Ingress TLS**: Encrypt traffic at ingress
- **Secrets Management**: Use external secrets operators

### Monitoring

Monitor NetworkPolicy denials:
- Check CNI plugin logs for dropped packets
- Use tools like Cilium Hubble for network observability
- Set up alerts for unexpected connection attempts

## Important Production Considerations

### Node IP and API Server Access

**⚠️ Important**: The example NetworkPolicies use `podSelector` and `namespaceSelector` which only match pods within the cluster. For production deployments, you may need to use `ipBlock` for:

1. **Kubernetes Health Probes**: Kubelet health probes originate from the node IP, not from pods. To allow health probes:
   ```yaml
   - from:
     - ipBlock:
         cidr: 10.0.0.0/8  # Replace with your node CIDR range
   ```

2. **Kubernetes API Server Access**: The API server is typically reached via node IPs or control-plane endpoints. For operator/scaler egress to API server:
   ```yaml
   - to:
     - ipBlock:
         cidr: 10.96.0.1/32  # kubernetes.default Service IP
   # OR specify your control-plane endpoint CIDRs
   ```

3. **External Services**: For application egress to external services (internet, external databases):
   ```yaml
   - to:
     - ipBlock:
         cidr: 0.0.0.0/0  # Allow all (not recommended)
         except:
           - 10.0.0.0/8   # Exclude internal ranges
   # OR specify exact external IP ranges needed
   ```

### Customization Required

These example policies are **templates** that require customization for your environment:

- **Ingress Controller Labels**: Update `interceptor-networkpolicy.yaml` to match your actual ingress controller's namespace and pod labels
- **Monitoring Labels**: Update metrics scraping rules to match your Prometheus/monitoring setup
- **IP Ranges**: Add `ipBlock` rules for node IPs, API server, and external services as needed
- **Namespace Labels**: Verify your namespaces have the labels referenced in `namespaceSelector` rules

## Production Recommendations

1. **Test in staging first**: Verify all functionality works with policies applied
2. **Start permissive**: Begin with broader rules, then tighten based on actual traffic
3. **Document exceptions**: If you need to add allow rules, document why
4. **Regular audits**: Review policies periodically as architecture evolves
5. **Backup policies**: Keep policies in version control

## Helm Chart Integration

These NetworkPolicies can be optionally enabled in the Helm chart. See the main Helm chart documentation for configuration options.

## References

- [Kubernetes NetworkPolicy Documentation](https://kubernetes.io/docs/concepts/services-networking/network-policies/)
- [KEDA HTTP Add-on Architecture](https://keda.sh/http-add-on/latest/)
- [NetworkPolicy Recipes](https://github.com/ahmetb/kubernetes-network-policy-recipes)

## Contributing

If you find issues or have improvements, please open an issue or pull request in the [KEDA HTTP Add-on repository](https://github.com/kedacore/http-add-on).
