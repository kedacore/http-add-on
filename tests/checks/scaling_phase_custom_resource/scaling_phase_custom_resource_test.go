//go:build e2e

package scaling_phase_test

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	. "github.com/kedacore/http-add-on/tests/helper"
)

const (
	testName = "scaling-phase-custom-resource-test"
)

var (
	testNamespace        = fmt.Sprintf("%s-ns", testName)
	rolloutName          = fmt.Sprintf("%s-rollout", testName)
	serviceName          = fmt.Sprintf("%s-service", testName)
	httpScaledObjectName = fmt.Sprintf("%s-http-so", testName)
	host                 = testName
	minReplicaCount      = 0
	maxReplicaCount      = 4
)

type templateData struct {
	TestNamespace        string
	RolloutName          string
	ServiceName          string
	HTTPScaledObjectName string
	Host                 string
	MinReplicas          int
	MaxReplicas          int
}

const (
	serviceTemplate = `
apiVersion: v1
kind: Service
metadata:
  name: {{.ServiceName}}
  namespace: {{.TestNamespace}}
  labels:
    app: {{.RolloutName}}
spec:
  ports:
    - port: 8080
      targetPort: http
      protocol: TCP
      name: http
  selector:
    app: {{.RolloutName}}
`

	workloadTemplate = `
apiVersion: argoproj.io/v1alpha1
kind: Rollout
metadata:
  name: {{.RolloutName}}
  namespace: {{.TestNamespace}}
  labels:
    app: {{.RolloutName}}
spec:
  replicas: 0
  strategy:
    canary:
      steps:
        - setWeight: 50
        - pause: {duration: 1}
  selector:
    matchLabels:
      app: {{.RolloutName}}
  template:
    metadata:
      labels:
        app: {{.RolloutName}}
    spec:
      containers:
        - name: {{.RolloutName}}
          image: registry.k8s.io/e2e-test-images/agnhost:2.45
          args:
          - netexec
          ports:
            - name: http
              containerPort: 8080
              protocol: TCP
          readinessProbe:
            httpGet:
              path: /
              port: http
`

	loadJobTemplate = `
apiVersion: batch/v1
kind: Job
metadata:
  name: load-generator
  namespace: {{.TestNamespace}}
spec:
  template:
    spec:
      containers:
      - name: apache-ab
        image: ghcr.io/kedacore/tests-apache-ab
        imagePullPolicy: Always
        args:
          - "-n"
          - "20000"
          - "-c"
          - "1"
          - "-H"
          - "Host: {{.Host}}"
          - "http://keda-add-ons-http-interceptor-proxy.keda:8080/"
      restartPolicy: Never
      terminationGracePeriodSeconds: 5
  activeDeadlineSeconds: 600
  backoffLimit: 5
`
	httpScaledObjectTemplate = `
kind: HTTPScaledObject
apiVersion: http.keda.sh/v1alpha1
metadata:
  name: {{.HTTPScaledObjectName}}
  namespace: {{.TestNamespace}}
spec:
  hosts:
  - {{.Host}}
  scalingMetric:
    requestRate:
      granularity: 1s
      targetValue: 2
      window: 1m
  scaledownPeriod: 0
  scaleTargetRef:
    name: {{.RolloutName}}
    apiVersion: argoproj.io/v1alpha1
    kind: Rollout
    service: {{.ServiceName}}
    port: 8080
  replicas:
    min: {{ .MinReplicas }}
    max: {{ .MaxReplicas }}
`
)

func TestCheck(t *testing.T) {
	// setup
	t.Log("--- setting up ---")
	// Create kubernetes resources
	kc := GetKubernetesClient(t)
	data, templates := getTemplateData()
	CreateKubernetesResources(t, kc, testNamespace, data, templates)

	assert.True(t, waitForArgoRolloutReplicaCount(t, rolloutName, testNamespace, minReplicaCount, 6, 10),
		"replica count should be %d after 1 minutes", minReplicaCount)

	testScaleOut(t, data)
	testScaleIn(t)

	// cleanup
	DeleteKubernetesResources(t, testNamespace, data, templates)
}

func testScaleOut(t *testing.T, data templateData) {
	t.Log("--- testing scale out ---")

	KubectlApplyWithTemplate(t, data, "loadJobTemplate", loadJobTemplate)

	assert.True(t, waitForArgoRolloutReplicaCount(t, rolloutName, testNamespace, maxReplicaCount, 18, 10),
		"replica count should be %d after 3 minutes", maxReplicaCount)
	KubectlDeleteWithTemplate(t, data, "loadJobTemplate", loadJobTemplate)
}

func testScaleIn(t *testing.T) {
	t.Log("--- testing scale out ---")

	assert.True(t, waitForArgoRolloutReplicaCount(t, rolloutName, testNamespace, minReplicaCount, 12, 10),
		"replica count should be %d after 2 minutes", minReplicaCount)
}

func getTemplateData() (templateData, []Template) {
	return templateData{
			TestNamespace:        testNamespace,
			RolloutName:          rolloutName,
			ServiceName:          serviceName,
			HTTPScaledObjectName: httpScaledObjectName,
			Host:                 host,
			MinReplicas:          minReplicaCount,
			MaxReplicas:          maxReplicaCount,
		}, []Template{
			{Name: "workloadTemplate", Config: workloadTemplate},
			{Name: "serviceNameTemplate", Config: serviceTemplate},
			{Name: "httpScaledObjectTemplate", Config: httpScaledObjectTemplate},
		}
}

func waitForArgoRolloutReplicaCount(t *testing.T, name, namespace string,
	target, iterations, intervalSeconds int,
) bool {
	for range iterations {
		kctlGetCmd := fmt.Sprintf(`kubectl get rollouts.argoproj.io/%s -n %s -o jsonpath="{.spec.replicas}"`, name, namespace)
		output, err := ExecuteCommand(kctlGetCmd)

		require.NoErrorf(t, err, "cannot get rollout info - %s", err)

		unqoutedOutput := strings.ReplaceAll(string(output), "\"", "")
		replicas, err := strconv.ParseInt(unqoutedOutput, 10, 64)
		require.NoErrorf(t, err, "cannot convert rollout count to int - %s", err)

		t.Logf("Waiting for rollout replicas to hit target. Deployment - %s, Current  - %d, Target - %d",
			name, replicas, target)

		if replicas == int64(target) {
			return true
		}

		time.Sleep(time.Duration(intervalSeconds) * time.Second)
	}

	return false
}
