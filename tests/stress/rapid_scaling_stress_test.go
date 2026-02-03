//go:build stress
// +build stress

package stress

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/kubernetes"

	. "github.com/kedacore/http-add-on/tests/helper"
)

const (
	rapidTestName = "rapid-scaling-stress-test"
)

var (
	rapidTestNamespace        = fmt.Sprintf("%s-ns", rapidTestName)
	rapidDeploymentName       = fmt.Sprintf("%s-deployment", rapidTestName)
	rapidServiceName          = fmt.Sprintf("%s-service", rapidTestName)
	rapidHTTPScaledObjectName = fmt.Sprintf("%s-http-so", rapidTestName)
	rapidHost                 = rapidTestName
	rapidMinReplicaCount      = 0
	rapidMaxReplicaCount      = 8
)

type rapidTemplateData struct {
	TestNamespace        string
	DeploymentName       string
	ServiceName          string
	HTTPScaledObjectName string
	Host                 string
	MinReplicas          int
	MaxReplicas          int
	JobName              string
	Requests             string
	Concurrency          string
}

const (
	rapidServiceTemplate = `
apiVersion: v1
kind: Service
metadata:
  name: {{.ServiceName}}
  namespace: {{.TestNamespace}}
  labels:
    app: {{.DeploymentName}}
spec:
  ports:
    - port: 8080
      targetPort: http
      protocol: TCP
      name: http
  selector:
    app: {{.DeploymentName}}
`

	rapidWorkloadTemplate = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{.DeploymentName}}
  namespace: {{.TestNamespace}}
  labels:
    app: {{.DeploymentName}}
spec:
  replicas: 0
  selector:
    matchLabels:
      app: {{.DeploymentName}}
  template:
    metadata:
      labels:
        app: {{.DeploymentName}}
    spec:
      containers:
        - name: {{.DeploymentName}}
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

	rapidLoadJobTemplate = `
apiVersion: batch/v1
kind: Job
metadata:
  name: {{.JobName}}
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
          - "{{.Requests}}"
          - "-c"
          - "{{.Concurrency}}"
          - "-H"
          - "Host: {{.Host}}"
          - "http://keda-add-ons-http-interceptor-proxy.keda:8080/"
      restartPolicy: Never
      terminationGracePeriodSeconds: 5
  activeDeadlineSeconds: 300
  backoffLimit: 5
`
	rapidHTTPScaledObjectTemplate = `
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
      targetValue: 20
      window: 30s
  scaledownPeriod: 5
  scaleTargetRef:
    name: {{.DeploymentName}}
    service: {{.ServiceName}}
    port: 8080
  replicas:
    min: {{ .MinReplicas }}
    max: {{ .MaxReplicas }}
`
)

func TestRapidScalingStress(t *testing.T) {
	// setup
	t.Log("--- setting up rapid scaling stress test ---")
	kc := GetKubernetesClient(t)
	data, templates := getRapidTemplateData()
	CreateKubernetesResources(t, kc, rapidTestNamespace, data, templates)

	assert.True(t, WaitForDeploymentReplicaReadyCount(t, kc, rapidDeploymentName, rapidTestNamespace, rapidMinReplicaCount, 6, 10),
		"replica count should be %d after 1 minute", rapidMinReplicaCount)

	// Run multiple rapid scale up/down cycles
	testRapidScalingCycles(t, kc, data)

	// cleanup
	DeleteKubernetesResources(t, rapidTestNamespace, data, templates)
}

func testRapidScalingCycles(t *testing.T, kc *kubernetes.Clientset, data rapidTemplateData) {
	cycles := 3
	for i := 1; i <= cycles; i++ {
		t.Logf("--- testing rapid scaling cycle %d of %d ---", i, cycles)

		// Generate unique job name for this cycle
		data.JobName = fmt.Sprintf("load-generator-%d", i)

		// Scale up phase
		t.Logf("--- cycle %d: scale up phase ---", i)
		KubectlApplyWithTemplate(t, data, "rapidLoadJobTemplate", rapidLoadJobTemplate)

		assert.True(t, WaitForDeploymentReplicaReadyMinCount(t, kc, rapidDeploymentName, rapidTestNamespace, rapidMaxReplicaCount-2, 18, 10),
			"replica count should reach near max (%d) in cycle %d", rapidMaxReplicaCount, i)

		KubectlDeleteWithTemplate(t, data, "rapidLoadJobTemplate", rapidLoadJobTemplate)

		// Scale down phase
		t.Logf("--- cycle %d: scale down phase ---", i)
		assert.True(t, WaitForDeploymentReplicaReadyCount(t, kc, rapidDeploymentName, rapidTestNamespace, rapidMinReplicaCount, 18, 10),
			"replica count should return to %d in cycle %d", rapidMinReplicaCount, i)

		// Brief pause between cycles
		if i < cycles {
			t.Logf("--- brief pause before next cycle ---")
			time.Sleep(10 * time.Second)
		}
	}

	t.Log("--- rapid scaling cycles completed successfully ---")
}

func getRapidTemplateData() (rapidTemplateData, []Template) {
	return rapidTemplateData{
			TestNamespace:        rapidTestNamespace,
			DeploymentName:       rapidDeploymentName,
			ServiceName:          rapidServiceName,
			HTTPScaledObjectName: rapidHTTPScaledObjectName,
			Host:                 rapidHost,
			MinReplicas:          rapidMinReplicaCount,
			MaxReplicas:          rapidMaxReplicaCount,
			JobName:              "load-generator-1",
			Requests:             "50000",
			Concurrency:          "50",
		}, []Template{
			{Name: "rapidWorkloadTemplate", Config: rapidWorkloadTemplate},
			{Name: "rapidServiceNameTemplate", Config: rapidServiceTemplate},
			{Name: "rapidHTTPScaledObjectTemplate", Config: rapidHTTPScaledObjectTemplate},
		}
}
