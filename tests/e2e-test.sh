#!/bin/bash

TAG=${TAG:-canary}

# Install KEDA
helm repo add kedacore https://kedacore.github.io/charts
helm repo update
helm upgrade --install keda kedacore/keda --namespace keda --create-namespace --wait

# Show Kubernetes resources in keda namespace
kubectl get all --namespace keda

# Install Http-add-on
helm upgrade --install http-add-on kedacore/keda-add-ons-http --set images.tag=$TAG --namespace keda --create-namespace --wait

# Show Kubernetes resources in http-add-on namespace
kubectl get all --namespace http-add-on

# Create resources
kubectl create ns app
helm upgrade --install xkcd ./examples/xkcd -n app --wait

# Show Kubernetes resources in app namespace
kubectl get all --namespace app

# Check http-add-on generated ScaledObject
n=0
max=5
until [ "$n" -ge "$max" ]
do
  ready=$(kubectl get so xkcd-app -n app -o jsonpath="{.status.conditions[0].status}")
  echo "ready: $ready"
  if [ $ready == "True" ]; then
    break
  fi
  n=$((n+1))
  sleep 15
done
if [ $n -eq $max ]; then
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
        command: ["curl", "-H", "Host: myhost.com", "keda-add-ons-http-interceptor-proxy.keda:8080"]
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
  echo "Current replica count is not 1"
  exit 1
fi

echo "The workflow is scaled to one"

echo "SUCCESS"
