//go:build e2e
// +build e2e

package utils

import (
	"fmt"
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	. "github.com/kedacore/http-add-on/tests/helper"
)

var (
	OtlpConfig = `mode: deployment
image:
  repository: "ghcr.io/open-telemetry/opentelemetry-collector-releases/opentelemetry-collector-contrib"
config:
  exporters:
    debug:
      verbosity: basic
    prometheus:
      endpoint: 0.0.0.0:8889
    zipkin:
      endpoint: http://zipkin.zipkin:9411/api/v2/spans
  receivers:
    jaeger: null
    prometheus: null
    zipkin: null
  service:
    pipelines:
      traces:
        receivers:
          - otlp
        exporters:
          - zipkin
      metrics:
        receivers:
          - otlp
        exporters:
          - debug
          - prometheus
      logs: null
`
	OtlpServicePatch = `apiVersion: v1
kind: Service
metadata:
  name: opentelemetry-collector
spec:
  selector:
    app.kubernetes.io/name: opentelemetry-collector
  ports:
    - protocol: TCP
      port: 8889
      targetPort: 8889
      name: prometheus
  type: ClusterIP

---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: opentelemetry-collector
spec:
  selector:
    matchLabels:
      app.kubernetes.io/name: opentelemetry-collector
  template:
    metadata:
      labels:
        app.kubernetes.io/name: opentelemetry-collector
    spec:
      containers:
      - name: opentelemetry-collector
        ports:
        - containerPort: 8889
          name: prometheus
          protocol: TCP
`

	gatewayClass = `
apiVersion: gateway.networking.k8s.io/v1
kind: GatewayClass
metadata:
  name: eg
spec:
  controllerName: gateway.envoyproxy.io/gatewayclass-controller
`

	gateway = `
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: eg
  namespace: envoy-gateway-system
spec:
  gatewayClassName: eg
  listeners:
    - name: http
      protocol: HTTP
      port: 80
      allowedRoutes:
        namespaces:
          from: All
`
	zipkinTemplate = `---
apiVersion: apps/v1
kind: Deployment
metadata:
  creationTimestamp: null
  labels:
    app: zipkin
  name: zipkin
  namespace: zipkin
spec:
  replicas: 1
  selector:
    matchLabels:
      app: zipkin
  strategy: {}
  template:
    metadata:
      creationTimestamp: null
      labels:
        app: zipkin
    spec:
      containers:
      - image: openzipkin/zipkin:3
        name: zipkin
        env:
        - name: "JAVA_OPTS"
          value: "-XX:MaxRAMPercentage=80 -XX:InitialRAMPercentage=80"
        resources:
          limits:
            memory: "256M"
          requests:
            memory: "256M"
---
apiVersion: v1
kind: Service
metadata:
  creationTimestamp: null
  labels:
    app: zipkin
  name: zipkin
  namespace: zipkin
spec:
  ports:
  - port: 9411
    protocol: TCP
    targetPort: 9411
  selector:
    app: zipkin
  type: ClusterIP
status:
  loadBalancer: {}
`
)

func TestVerifyCommands(t *testing.T) {
	commands := []string{"kubectl"}
	for _, cmd := range commands {
		_, err := exec.LookPath(cmd)
		require.NoErrorf(t, err, "%s is required for setup - %s", cmd, err)
	}
}

func TestKubernetesConnection(t *testing.T) {
	KubeClient = GetKubernetesClient(t)
}

func TestKubernetesVersion(t *testing.T) {
	out, err := ExecuteCommand("kubectl version")
	require.NoErrorf(t, err, "error getting kubernetes version - %s", err)

	t.Logf("kubernetes version: %s", string(out))
}

func TestSetupHelm(t *testing.T) {
	_, err := exec.LookPath("helm")
	if err == nil {
		t.Skip("helm is already installed. skipping setup.")
	}

	_, err = ExecuteCommand("curl -fsSL -o get_helm.sh https://raw.githubusercontent.com/helm/helm/master/scripts/get-helm-3")
	require.NoErrorf(t, err, "cannot download helm installation shell script - %s", err)

	_, err = ExecuteCommand("chmod 700 get_helm.sh")
	require.NoErrorf(t, err, "cannot change permissions for helm installation script - %s", err)

	_, err = ExecuteCommand("./get_helm.sh")
	require.NoErrorf(t, err, "cannot download helm - %s", err)

	_, err = ExecuteCommand("helm version")
	require.NoErrorf(t, err, "cannot get helm version - %s", err)
}

func TestCreateKEDANamespace(t *testing.T) {
	KubeClient = GetKubernetesClient(t)
	CreateNamespace(t, KubeClient, KEDANamespace)
}

func TestSetupArgoRollouts(t *testing.T) {
	KubeClient = GetKubernetesClient(t)
	CreateNamespace(t, KubeClient, ArgoRolloutsNamespace)
	_, err := ExecuteCommand("helm version")
	require.NoErrorf(t, err, "helm is not installed - %s", err)

	_, err = ExecuteCommand("helm repo add argo https://argoproj.github.io/argo-helm")
	require.NoErrorf(t, err, "cannot add argo helm repo - %s", err)

	_, err = ExecuteCommand("helm repo update argo")
	require.NoErrorf(t, err, "cannot update argo helm repo - %s", err)

	_, err = ExecuteCommand(fmt.Sprintf("helm upgrade --install %s argo/argo-rollouts --namespace %s --wait",
		ArgoRolloutsName,
		ArgoRolloutsNamespace))
	require.NoErrorf(t, err, "cannot install argo-rollouts - %s", err)
}

func TestSetupIngress(t *testing.T) {
	KubeClient = GetKubernetesClient(t)
	CreateNamespace(t, KubeClient, IngressNamespace)
	_, err := ExecuteCommand("helm version")
	require.NoErrorf(t, err, "helm is not installed - %s", err)

	_, err = ExecuteCommand("helm repo add ingress-nginx https://kubernetes.github.io/ingress-nginx")
	require.NoErrorf(t, err, "cannot add ingress-nginx helm repo - %s", err)

	_, err = ExecuteCommand("helm repo update ingress-nginx")
	require.NoErrorf(t, err, "cannot update ingress-nginx helm repo - %s", err)

	_, err = ExecuteCommand(fmt.Sprintf("helm upgrade --install %s ingress-nginx/ingress-nginx --set fullnameOverride=%s --set controller.service.type=ClusterIP --set controller.progressDeadlineSeconds=30 --namespace %s --wait",
		IngressReleaseName, IngressReleaseName, IngressNamespace))
	require.NoErrorf(t, err, "cannot install ingress - %s", err)
}

func TestSetupEnvoyGateway(t *testing.T) {
	KubeClient = GetKubernetesClient(t)
	_, err := ExecuteCommand("helm version")
	require.NoErrorf(t, err, "helm is not installed - %s", err)

	_, err = ExecuteCommand(fmt.Sprintf("helm install %s oci://docker.io/envoyproxy/gateway-helm --version v1.2.0 -n %s --create-namespace", EnvoyReleaseName, EnvoyNamespace))
	require.NoErrorf(t, err, "cannot install envoy gateway - %s", err)

	assert.True(t, WaitForDeploymentReplicaReadyCount(t, KubeClient, "envoy-gateway", "envoy-gateway-system", 1, 30, 6))

	KubectlApplyWithTemplate(t, nil, "gatewayClass", gatewayClass)
	KubectlApplyWithTemplate(t, nil, "gateway", gateway)
}

func TestSetupKEDA(t *testing.T) {
	_, err := ExecuteCommand("helm version")
	require.NoErrorf(t, err, "helm is not installed - %s", err)

	_, err = ExecuteCommand("helm repo add kedacore https://kedacore.github.io/charts")
	require.NoErrorf(t, err, "cannot add kedacore helm repo - %s", err)

	_, err = ExecuteCommand("helm repo update kedacore")
	require.NoErrorf(t, err, "cannot update kedacore helm repo - %s", err)

	_, err = ExecuteCommand(fmt.Sprintf("helm upgrade --install keda kedacore/keda --namespace %s --set extraArgs.keda.kube-api-qps=200 --set extraArgs.keda.kube-api-burst=300",
		KEDANamespace))
	require.NoErrorf(t, err, "cannot install KEDA - %s", err)

	KubeClient = GetKubernetesClient(t)
	assert.True(t, WaitForDeploymentReplicaReadyCount(t, KubeClient, "keda-operator", KEDANamespace, 1, 30, 6),
		"replica count should be 1 after 3 minutes")
	assert.True(t, WaitForDeploymentReplicaReadyCount(t, KubeClient, "keda-operator-metrics-apiserver", KEDANamespace, 1, 30, 6),
		"replica count should be 1 after 3 minutes")
	assert.True(t, WaitForDeploymentReplicaReadyCount(t, KubeClient, "keda-admission-webhooks", KEDANamespace, 1, 30, 6),
		"replica count should be 1 after 3 minutes")
}

func TestSetupOpentelemetryComponents(t *testing.T) {
	OpentelemetryNamespace := "open-telemetry-system"
	otlpTempFileName := "otlp.yml"
	otlpServiceTempFileName := "otlpServicePatch.yml"
	defer os.Remove(otlpTempFileName)
	defer os.Remove(otlpServiceTempFileName)
	err := os.WriteFile(otlpTempFileName, []byte(OtlpConfig), 0755)
	assert.NoErrorf(t, err, "cannot create otlp config file - %s", err)

	err = os.WriteFile(otlpServiceTempFileName, []byte(OtlpServicePatch), 0755)
	assert.NoErrorf(t, err, "cannot create otlp service patch file - %s", err)

	_, err = ExecuteCommand("helm version")
	require.NoErrorf(t, err, "helm is not installed - %s", err)

	_, err = ExecuteCommand("helm repo add open-telemetry https://open-telemetry.github.io/opentelemetry-helm-charts")
	require.NoErrorf(t, err, "cannot add open-telemetry helm repo - %s", err)

	_, err = ExecuteCommand("helm repo update open-telemetry")
	require.NoErrorf(t, err, "cannot update open-telemetry helm repo - %s", err)

	KubeClient = GetKubernetesClient(t)
	CreateNamespace(t, KubeClient, OpentelemetryNamespace)

	_, err = ExecuteCommand(fmt.Sprintf("helm upgrade --install opentelemetry-collector open-telemetry/opentelemetry-collector -f %s --namespace %s", otlpTempFileName, OpentelemetryNamespace))
	require.NoErrorf(t, err, "cannot install opentelemetry - %s", err)

	_, err = ExecuteCommand(fmt.Sprintf("kubectl apply -f %s -n %s", otlpServiceTempFileName, OpentelemetryNamespace))
	require.NoErrorf(t, err, "cannot update opentelemetry ports - %s", err)
}

func TestDeployZipkin(t *testing.T) {
	KubeClient = GetKubernetesClient(t)
	CreateNamespace(t, KubeClient, "zipkin")

	zipkinTemplateFileName := "otlpServicePatch.yml"
	defer os.Remove(zipkinTemplateFileName)
	err := os.WriteFile(zipkinTemplateFileName, []byte(zipkinTemplate), 0755)
	assert.NoErrorf(t, err, "cannot create otlp config file - %s", err)

	_, err = ExecuteCommand(fmt.Sprintf("kubectl apply -f  %s -n %s", zipkinTemplateFileName, "zipkin"))
	require.NoErrorf(t, err, "cannot deploy zipkin - %s", err)

	assert.True(t, WaitForDeploymentReplicaReadyCount(t, KubeClient, "zipkin", "zipkin", 1, 6, 10),
		"replica count should be 1 after 3 minutes")
}

func TestSetupTLSConfiguration(t *testing.T) {
	out, err := ExecuteCommandWithDir("make test-certs", "../..")
	require.NoErrorf(t, err, "error generating test certs - %s", err)
	t.Log(string(out))
	t.Log("test certificates successfully generated")

	_, err = ExecuteCommand("kubectl -n keda create secret tls keda-tls --cert ../../certs/tls.crt --key ../../certs/tls.key")
	require.NoErrorf(t, err, "could not create tls cert secret in keda namespace - %s", err)

	_, err = ExecuteCommand("kubectl -n keda create secret tls abc-certs --cert ../../certs/abc.tls.crt --key ../../certs/abc.tls.key")
	require.NoErrorf(t, err, "could not create tls cert secret in keda namespace - %s", err)
}

func TestDeployKEDAHttpAddOn(t *testing.T) {
	out, err := ExecuteCommandWithDir("make deploy-e2e", "../..")
	require.NoErrorf(t, err, "error deploying KEDA Http Add-on - %s", err)

	t.Log(string(out))
	t.Log("KEDA Http Add-on deployed successfully using 'make deploy' command")
}
