//go:build e2e

package scaling_phase_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/kubernetes"

	. "github.com/kedacore/http-add-on/tests/helper"
)

const (
	testName = "scaling-phase-statefulset-test"
)

var (
	testNamespace        = fmt.Sprintf("%s-ns", testName)
	statefulSetName      = fmt.Sprintf("%s-statefulset", testName)
	serviceName          = fmt.Sprintf("%s-service", testName)
	interceptorRouteName = fmt.Sprintf("%s-ir", testName)
	scaledObjectName     = fmt.Sprintf("%s-so", testName)
	host                 = testName
	minReplicaCount      = 0
	maxReplicaCount      = 4
)

type templateData struct {
	TestNamespace        string
	StatefulSetName      string
	ServiceName          string
	InterceptorRouteName string
	ScaledObjectName     string
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
    app: {{.StatefulSetName}}
spec:
  ports:
    - port: 8080
      targetPort: http
      protocol: TCP
      name: http
  selector:
    app: {{.StatefulSetName}}
`

	workloadTemplate = `
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: {{.StatefulSetName}}
  namespace: {{.TestNamespace}}
  labels:
    app: {{.StatefulSetName}}
spec:
  replicas: 0
  selector:
    matchLabels:
      app: {{.StatefulSetName}}
  template:
    metadata:
      labels:
        app: {{.StatefulSetName}}
    spec:
      containers:
        - name: {{.StatefulSetName}}
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
	interceptorRouteTemplate = `
kind: InterceptorRoute
apiVersion: http.keda.sh/v1beta1
metadata:
  name: {{.InterceptorRouteName}}
  namespace: {{.TestNamespace}}
spec:
  target:
    service: {{.ServiceName}}
    port: 8080
  scalingMetric:
    requestRate:
      granularity: 1s
      targetValue: 2
      window: 1m
  rules:
  - hosts:
    - {{.Host}}
`

	scaledObjectTemplate = `
apiVersion: keda.sh/v1alpha1
kind: ScaledObject
metadata:
  name: {{.ScaledObjectName}}
  namespace: {{.TestNamespace}}
spec:
  cooldownPeriod: 0
  maxReplicaCount: {{ .MaxReplicas }}
  minReplicaCount: {{ .MinReplicas }}
  scaleTargetRef:
    apiVersion: apps/v1
    kind: StatefulSet
    name: {{.StatefulSetName}}
  triggers:
  - type: external-push
    metadata:
      interceptorRoute: {{.InterceptorRouteName}}
      scalerAddress: keda-add-ons-http-external-scaler.keda:9090
`
)

func TestCheck(t *testing.T) {
	// setup
	t.Log("--- setting up ---")
	// Create kubernetes resources
	kc := GetKubernetesClient(t)
	data, templates := getTemplateData()
	CreateKubernetesResources(t, kc, testNamespace, data, templates)

	assert.True(t, WaitForStatefulsetReplicaReadyCount(t, kc, statefulSetName, testNamespace, minReplicaCount, 6, 10),
		"replica count should be %d after 1 minutes", minReplicaCount)

	testScaleOut(t, kc, data)
	testScaleIn(t, kc)

	// cleanup
	DeleteKubernetesResources(t, testNamespace, data, templates)
}

func testScaleOut(t *testing.T, kc *kubernetes.Clientset, data templateData) {
	t.Log("--- testing scale out ---")

	KubectlApplyWithTemplate(t, data, "loadJobTemplate", loadJobTemplate)

	assert.True(t, WaitForStatefulsetReplicaReadyCount(t, kc, statefulSetName, testNamespace, maxReplicaCount, 18, 10),
		"replica count should be %d after 3 minutes", maxReplicaCount)
	KubectlDeleteWithTemplate(t, data, "loadJobTemplate", loadJobTemplate)
}

func testScaleIn(t *testing.T, kc *kubernetes.Clientset) {
	t.Log("--- testing scale out ---")

	assert.True(t, WaitForStatefulsetReplicaReadyCount(t, kc, statefulSetName, testNamespace, minReplicaCount, 12, 10),
		"replica count should be %d after 2 minutes", minReplicaCount)
}

func getTemplateData() (templateData, []Template) {
	return templateData{
			TestNamespace:        testNamespace,
			StatefulSetName:      statefulSetName,
			ServiceName:          serviceName,
			InterceptorRouteName: interceptorRouteName,
			ScaledObjectName:     scaledObjectName,
			Host:                 host,
			MinReplicas:          minReplicaCount,
			MaxReplicas:          maxReplicaCount,
		}, []Template{
			{Name: "workloadTemplate", Config: workloadTemplate},
			{Name: "serviceNameTemplate", Config: serviceTemplate},
			{Name: "interceptorRouteTemplate", Config: interceptorRouteTemplate},
			{Name: "scaledObjectTemplate", Config: scaledObjectTemplate},
		}
}
