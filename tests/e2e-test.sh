#!/bin/bash

function clear_resources(){
  # Clear e2e resources
  kubectl delete ns app

  # Remove Http-add-on
  make undeploy

  # Remove KEDA
  helm uninstall keda --namespace keda --wait
}

function print_logs {
  echo ">>> KEDA Operator log <<<"
  kubectl logs -n keda -l app=keda-operator --tail 5000
  printf "##############################################\n"
  printf "##############################################\n"

  echo ">>> HTTP Add-on Operator log <<<"
  kubectl logs -n keda -l app.kubernetes.io/instance=operator -c operator --tail 5000
  printf "##############################################\n"
  printf "##############################################\n"

  echo ">>> HTTP Add-on Interceptor log <<<"
  kubectl logs -n keda -l app.kubernetes.io/instance=interceptor --tail 5000
  printf "##############################################\n"
  printf "##############################################\n"

    echo ">>> HTTP Add-on Scaler log <<<"
  kubectl logs -n keda -l app.kubernetes.io/instance=external-scaler --tail 5000
  printf "##############################################\n"
  printf "##############################################\n"
}

# Install KEDA
helm repo add kedacore https://kedacore.github.io/charts
helm repo update
helm upgrade --install keda kedacore/keda --namespace keda --create-namespace --wait

# Install Http-add-on
make deploy

# Give a minute for pods to become ready
sleep 60

# Show Kubernetes resources in keda namespace
kubectl get all --namespace keda

# Create resources
kubectl create ns app
helm upgrade --install xkcd ./examples/xkcd -n app --wait

# Give a minute for resources to be created
sleep 60

# Show Kubernetes resources in app namespace
kubectl get all,httpso,so --namespace app

# Check http-add-on generated ScaledObject
n=0
max=5
until [ "$n" -ge "$max" ]
do
  ready=$(kubectl get so xkcd -n app -o jsonpath="{.status.conditions[0].status}")
  echo "ready: $ready"
  if [ "$ready" == "True" ]; then
    break
  fi
  n=$((n+1))
  sleep 15
done
if [ $n -eq $max ]; then
  kubectl get all --namespace keda
  print_logs
  clear_resources
  echo "The ScaledObject is not working correctly"
  exit 1
fi
echo "The ScaledObject is working correctly"

# Check that deployment has 0 instances
n=0
max=5
until [ "$n" -ge "$max" ]
do
  replicas=$(kubectl get deploy xkcd -n app -o jsonpath="{.spec.replicas}")
  echo "replicas: $replicas"
  if [ $replicas == "0" ]; then
    break
  fi
  n=$((n+1))
  sleep 15
done
if [ $n -eq $max ]; then
  echo "Current replica count is not 0"
  exit 1
fi
echo "The workflow is scaled to zero"

# Generate one request
cat <<EOF | kubectl apply -f -
apiVersion: batch/v1
kind: Job
metadata:
  name: generate-request
  namespace: app
spec:
  template:
    spec:
      containers:
      - name: curl-client
        image: curlimages/curl
        imagePullPolicy: Always
        command: ["curl", "-H", "Host: myhost.com", "keda-http-add-on-interceptor-proxy.keda:8080"]
      restartPolicy: Never
  activeDeadlineSeconds: 600
  backoffLimit: 5
EOF

# Check that deployment has 1 instances
n=0
max=5
until [ "$n" -ge "$max" ]
do
  replicas=$(kubectl get deploy xkcd -n app -o jsonpath="{.spec.replicas}")
  echo "replicas: $replicas"
  if [ $replicas == "1" ]; then
    break
  fi
  n=$((n+1))
  sleep 15
done

if [ $n -eq $max ]; then
  print_logs
  clear_resources
  echo "Current replica count is not 1"
  exit 1
fi

clear_resources

echo "The workflow is scaled to one"
echo "SUCCESS"
