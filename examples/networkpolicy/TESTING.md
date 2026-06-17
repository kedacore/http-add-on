# Testing NetworkPolicy Examples

This guide provides step-by-step instructions for testing and verifying the NetworkPolicy examples for KEDA HTTP Add-on.

# Prerequisites

1. Kubernetes cluster with NetworkPolicy support
   - CNI plugin that supports NetworkPolicies (Calico, Cilium, Weave, etc.)
   - Verify with: `kubectl get networkpolicies --all-namespaces`

2. KEDA HTTP Add-on installed
   ```bash
   helm repo add kedacore https://kedacore.github.io/charts
   helm install keda-http-add-on kedacore/keda-add-ons-http -n keda --create-namespace
   ```

3. Test application deployed
   ```bash
   kubectl create namespace test-app
   # Use helm to install the xkcd example
   helm install xkcd examples/xkcd -n test-app
   ```

# Testing Strategy

The testing approach follows the principle: "Verify it works WITH policies, then verify it FAILS WITHOUT specific rules"

# Phase 1: Baseline (No NetworkPolicies)

First, verify everything works without NetworkPolicies:

```bash
# 1. Check all pods are running
kubectl get pods -n keda
kubectl get pods -n test-app
# Note: test-app will have 0 pods initially (scale-from-zero)

# 2. Test HTTP request flow via port-forward
kubectl port-forward -n keda svc/keda-add-ons-http-interceptor-proxy 8080:8080 &
sleep 2

# Send request with Host header matching HTTPScaledObject config
curl -H "Host: myhost.com" http://localhost:8080/path1

# Watch pods scale up from 0 to 1
kubectl get pods -n test-app -w
# Press Ctrl+C after pod appears

# Stop port-forward
pkill -f "port-forward.*8080:8080"

# 3. Check KEDA scaling works
kubectl get httpscaledobject -n test-app
kubectl describe httpscaledobject xkcd -n test-app

# 4. Verify metrics are collected
kubectl logs -n keda deployment/keda-add-ons-http-external-scaler --tail=50
```

Expected: All components work, requests succeed, scaling functions properly.

# Phase 2: Apply NetworkPolicies

Apply all NetworkPolicies:

```bash
kubectl apply -f examples/networkpolicy/interceptor-networkpolicy.yaml
kubectl apply -f examples/networkpolicy/operator-networkpolicy.yaml
kubectl apply -f examples/networkpolicy/scaler-networkpolicy.yaml
kubectl apply -f examples/networkpolicy/app-networkpolicy.yaml
```

Verify policies are applied:

```bash
kubectl get networkpolicies -n keda
kubectl get networkpolicies -n test-app
```

# Phase 3: Verify Functionality WITH Policies

Repeat all baseline tests:

```bash
# 1. Test HTTP request flow still works
kubectl port-forward -n keda svc/keda-add-ons-http-interceptor-proxy 8080:8080 &
sleep 2
curl -H "Host: myhost.com" http://localhost:8080/path1
pkill -f "port-forward.*8080:8080"

# 2. Verify scaling still works
kubectl get httpscaledobject -n test-app
kubectl get pods -n test-app

# 3. Check component logs for errors
kubectl logs -n keda deployment/keda-add-ons-http-interceptor --tail=50
kubectl logs -n keda deployment/keda-add-ons-http-controller-manager --tail=50
kubectl logs -n keda deployment/keda-add-ons-http-external-scaler --tail=50
```

Expected: Everything still works exactly as before.

# Phase 4: Verify Policies Block Unauthorized Traffic

Test that policies actually block unwanted traffic:

# Test 1: Block Direct Access to Interceptor Admin API

```bash
# Create a test pod in different namespace
kubectl run test-pod --image=curlimages/curl -n default --rm -it -- sh

# Try to access interceptor admin API (should FAIL)
curl http://keda-add-ons-http-interceptor-admin.keda:9090/queue
# Expected: Connection timeout or refused
```

# Test 2: Block Direct Access to Scaler

```bash
# From test pod, try to access scaler (should FAIL)
curl http://keda-add-ons-http-external-scaler.keda:9090
# Expected: Connection timeout or refused
```

# Test 3: Block Application from Unauthorized Sources

```bash
# Create pod in unauthorized namespace
kubectl run unauthorized-pod --image=curlimages/curl -n unauthorized --rm -it -- sh

# Try to access application directly (should FAIL)
curl http://example-app.test-app:8080
# Expected: Connection timeout or refused
```

# Phase 5: Verify Specific Rules

# Test Interceptor → Backend Communication

```bash
# For kind/local clusters, use port-forward
kubectl port-forward -n keda svc/keda-add-ons-http-interceptor-proxy 8080:8080 &
sleep 2

# Send request through interceptor
curl -H "Host: myhost.com" http://localhost:8080/path1

# Stop port-forward
pkill -f "port-forward.*8080:8080"

# Check interceptor logs show successful proxy
kubectl logs -n keda deployment/keda-add-ons-http-interceptor --tail=50 | grep -i "request\|proxy"
```

Expected: Request succeeds, logs show proxying to backend.

Note: For production clusters with LoadBalancer, replace `localhost:8080` with your ingress IP.

# Test Scaler → Interceptor Communication

```bash
# Check scaler can fetch queue counts
kubectl logs -n keda deployment/keda-add-ons-http-external-scaler | grep "queue count"
```

**Expected**: Logs show successful queue count fetches.

# Test KEDA → Scaler Communication

```bash
# Check KEDA operator logs
kubectl logs -n keda deployment/keda-operator | grep "external-scaler"
```

Expected: Logs show successful metric fetches.

# Detailed Test Cases

# Test Case 1: HTTP Traffic Flow

Objective: Verify end-to-end HTTP request flow works with NetworkPolicies.

Steps:
1. Apply all NetworkPolicies
2. Send HTTP request: `curl http://<ingress-ip>/test`
3. Check response is successful
4. Verify in logs:
   - Ingress → Interceptor
   - Interceptor → Application
   - Application → Response

Expected Result: Request succeeds with 200 OK.

Troubleshooting:
- If fails, check ingress controller labels match policy
- Verify interceptor can reach application pods
- Check DNS resolution works

# Test Case 2: Scaling Metrics Collection

Objective: Verify KEDA can collect metrics from Scaler.

Steps:
1. Apply all NetworkPolicies
2. Generate load: `hey -z 30s -c 10 http://<ingress-ip>/test`
3. Watch HPA: `kubectl get hpa -n test-app -w`
4. Verify scaling occurs

Expected Result: HPA shows current metrics, pods scale up/down.

Troubleshooting:
- Check KEDA operator can reach scaler
- Verify scaler can reach interceptor admin API
- Check scaler logs for errors

# Test Case 3: Operator CRD Management

Objective: Verify Operator can manage HTTPScaledObjects.

Steps:
1. Apply all NetworkPolicies
2. Create new HTTPScaledObject:
   ```bash
   kubectl apply -f - <<EOF
   apiVersion: http.keda.sh/v1alpha1
   kind: HTTPScaledObject
   metadata:
     name: test-hso
     namespace: test-app
   spec:
     hosts:
       - test.example.com
     scaleTargetRef:
       name: test-app
       kind: Deployment
     replicas:
       min: 1
       max: 10
   EOF
   ```
3. Verify ScaledObject is created: `kubectl get scaledobject -n test-app`

Expected Result: ScaledObject created successfully.

Troubleshooting:
- Check operator can access Kubernetes API
- Verify operator logs for errors
- Check RBAC permissions

# Test Case 4: DNS Resolution

Objective: Verify all components can resolve DNS.

Steps:
1. Apply all NetworkPolicies
2. Check each component can resolve services:
   ```bash
   # Interceptor
   kubectl exec -n keda deployment/keda-add-ons-http-interceptor -- nslookup kubernetes.default

   # Operator
   kubectl exec -n keda deployment/keda-add-ons-http-operator -- nslookup kubernetes.default

   # Scaler
   kubectl exec -n keda deployment/keda-add-ons-http-external-scaler -- nslookup kubernetes.default
   ```

Expected Result: All DNS lookups succeed.

Troubleshooting:
- Verify CoreDNS/kube-dns labels match policy
- Check DNS egress rules are correct

# Test Case 5: Metrics Scraping (Optional)

Objective: Verify Prometheus can scrape metrics if monitoring is enabled.

Steps:
1. Apply NetworkPolicies with Prometheus rules uncommented
2. Check Prometheus targets: `kubectl port-forward -n monitoring svc/prometheus 9090:9090`
3. Visit `http://localhost:9090/targets` in your browser
4. Verify HTTP Add-on targets are UP

Expected Result: All metrics endpoints are reachable.

Troubleshooting:
- Verify Prometheus namespace/labels match policy
- Check metrics ports are correct

# Verification Checklist

Use this checklist to verify all functionality:

- [ ] All pods running in keda namespace
- [ ] All NetworkPolicies applied successfully
- [ ] HTTP requests through interceptor succeed
- [ ] Application pods receive traffic from interceptor
- [ ] Scaler can fetch queue counts from interceptor
- [ ] KEDA can fetch metrics from scaler
- [ ] Operator can create/update ScaledObjects
- [ ] HPA shows current metrics
- [ ] Scaling up/down works correctly
- [ ] DNS resolution works for all components
- [ ] Unauthorized access is blocked
- [ ] Metrics scraping works (if enabled)
- [ ] No errors in component logs

# Common Issues and Solutions

# Issue 1: Connection Timeouts

Symptom: Requests timeout after applying NetworkPolicies.

Possible Causes:
- CNI doesn't support NetworkPolicies
- Labels don't match selectors
- Namespace labels missing

Solution:
```bash
# Check CNI supports NetworkPolicies
kubectl get nodes -o wide

# Verify pod labels
kubectl get pods -n keda --show-labels

# Add namespace labels if missing
kubectl label namespace keda name=keda
kubectl label namespace kube-system name=kube-system
```

# Issue 2: Scaling Doesn't Work

Symptom: HPA shows unknown metrics, pods don't scale.

Possible Causes:
- Scaler can't reach interceptor
- KEDA can't reach scaler
- API server access blocked

Solution:
```bash
# Check scaler logs
kubectl logs -n keda deployment/keda-add-ons-http-external-scaler

# Verify scaler can reach interceptor
kubectl exec -n keda deployment/keda-add-ons-http-external-scaler -- \
  curl http://keda-add-ons-http-interceptor-admin:9090/queue

# Check KEDA operator logs
kubectl logs -n keda deployment/keda-operator | grep external-scaler
```

# Issue 3: DNS Resolution Fails

Symptom: Components can't resolve service names.

Possible Causes:
- DNS egress rules missing
- CoreDNS labels don't match

Slution:
```bash
# Check CoreDNS labels
kubectl get pods -n kube-system -l k8s-app=kube-dns --show-labels

# Test DNS from pod
kubectl exec -n keda deployment/keda-add-ons-http-interceptor -- nslookup kubernetes.default

# Update NetworkPolicy with correct DNS labels
```

# Issue 4: Ingress Traffic Blocked

Symptom: External requests don't reach interceptor.

Possible Causes:
- Ingress controller labels don't match
- Ingress namespace not labeled

Solution:
```bash
# Check ingress controller labels
kubectl get pods -n ingress-nginx --show-labels

# Update interceptor NetworkPolicy with correct labels
# Add namespace label
kubectl label namespace ingress-nginx name=ingress-nginx
```

# Performance Testing

After verifying functionality, test performance impact:

```bash
# Baseline (no NetworkPolicies)
hey -z 60s -c 50 http://<ingress-ip>/test > baseline.txt

# With NetworkPolicies
kubectl apply -f examples/networkpolicy/
hey -z 60s -c 50 http://<ingress-ip>/test > with-policies.txt

# Compare results
diff baseline.txt with-policies.txt
```

Expected: Minimal performance impact (<5% latency increase).

# Cleanup

Remove NetworkPolicies:

```bash
kubectl delete -f examples/networkpolicy/ -n keda
kubectl delete -f examples/networkpolicy/app-networkpolicy.yaml -n test-app
```

# Next Steps

After successful testing:

1. Document any customizations needed for your environment
2. Consider integrating into Helm chart
3. Set up monitoring for NetworkPolicy denials
4. Create runbooks for troubleshooting

# References

- [Kubernetes NetworkPolicy Documentation](https://kubernetes.io/docs/concepts/services-networking/network-policies/)
- [NetworkPolicy Recipes](https://github.com/ahmetb/kubernetes-network-policy-recipes)
- [KEDA HTTP Add-on Documentation](https://keda.sh/http-add-on/latest/)
