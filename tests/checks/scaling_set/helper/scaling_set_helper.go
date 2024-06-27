//go:build e2e
// +build e2e

package scaling_set_helper

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/kubernetes"

	. "github.com/kedacore/http-add-on/tests/helper"
)

var (
	minReplicaCount = 0
	maxReplicaCount = 2
)

type TemplateData struct {
	TestNamespace            string
	DeploymentName           string
	ServiceName              string
	HTTPScaledObjectName     string
	HTTPScalingSetName       string
	HTTPScalingSetKind       string
	HTTPInterceptorService   string
	HTTPInterceptorNamespace string
	ClusterScoped            bool
	Host                     string
	MinReplicas              int
	MaxReplicas              int
}

const (
	serviceTemplate = `
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

	deploymentTemplate = `
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

	loadJobTemplate = `
apiVersion: batch/v1
kind: Job
metadata:
  name: generate-request
  namespace: {{.TestNamespace}}
spec:
  template:
    spec:
      terminationGracePeriodSeconds: 0
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
          - "http://{{.HTTPInterceptorService}}.{{.HTTPInterceptorNamespace}}:8080/"
      restartPolicy: Never
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
  scalingSet:
    name: {{.HTTPScalingSetName}}
    kind: {{.HTTPScalingSetKind}}
  hosts:
  - {{.Host}}
  scalingMetric:
    requestRate:
      granularity: 1s
      targetValue: 2
      window: 30s
  scaledownPeriod: 0
  scaleTargetRef:
    name: {{.DeploymentName}}
    service: {{.ServiceName}}
    port: 8080
  replicas:
    min: {{ .MinReplicas }}
    max: {{ .MaxReplicas }}
`

	httpClusterScalingSetTemplate = `
kind: ClusterHTTPScalingSet
apiVersion: http.keda.sh/v1alpha1
metadata:
  name: {{.HTTPScalingSetName}}
spec:
  interceptor:
    replicas: 1
    serviceAccountName: keda-http-add-on-interceptor
  scaler:
    serviceAccountName: keda-http-add-on-scaler
`

	httpScalingSetServiceAccount = `
apiVersion: v1
kind: ServiceAccount
metadata:
  name: {{.HTTPScalingSetName}}
  namespace: {{.TestNamespace}}
`
	httpScalingSetClusterRole = `
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: {{.HTTPScalingSetName}}
rules:
- apiGroups:
  - ""
  resources:
  - endpoints
  verbs:
  - get
  - list
  - watch
- apiGroups:
  - http.keda.sh
  resources:
  - httpscaledobjects
  verbs:
  - get
  - list
  - watch
`

	httpScalingSetClusterRoleBinding = `
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: {{.HTTPScalingSetName}}
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: {{.HTTPScalingSetName}}
subjects:
- kind: ServiceAccount
  name: {{.HTTPScalingSetName}}
  namespace: {{.TestNamespace}}
  `

	httpScalingSetTemplate = `
kind: HTTPScalingSet
apiVersion: http.keda.sh/v1alpha1
metadata:
  name: {{.HTTPScalingSetName}}
  namespace: {{.TestNamespace}}
spec:
  interceptor:
    replicas: 1
    serviceAccountName: {{.HTTPScalingSetName}}
    image: docker.io/jorturfer/http-add-on-interceptor:isolate
  scaler:
    serviceAccountName: {{.HTTPScalingSetName}}
    image: docker.io/jorturfer/http-add-on-scaler:isolate
`
)

func GetTemplateData(testName string, clusterScoped bool) (TemplateData, []Template) {
	namespace := fmt.Sprintf("%s-ns", testName)
	deploymentName := fmt.Sprintf("%s-deployment", testName)
	serviceName := fmt.Sprintf("%s-service", testName)
	httpScaledObjectName := fmt.Sprintf("%s-http-so", testName)
	scalingSetName := fmt.Sprintf("%s-ss", testName)
	interceptorServiceName := fmt.Sprintf("%s-interceptor-proxy", scalingSetName)

	templateData := TemplateData{
		TestNamespace:            namespace,
		DeploymentName:           deploymentName,
		ServiceName:              serviceName,
		HTTPScaledObjectName:     httpScaledObjectName,
		HTTPScalingSetName:       scalingSetName,
		HTTPScalingSetKind:       "HTTPScalingSet",
		Host:                     testName,
		MinReplicas:              minReplicaCount,
		MaxReplicas:              maxReplicaCount,
		ClusterScoped:            clusterScoped,
		HTTPInterceptorService:   interceptorServiceName,
		HTTPInterceptorNamespace: namespace,
	}

	templates := []Template{
		{Name: "deploymentTemplate", Config: deploymentTemplate},
		{Name: "serviceNameTemplate", Config: serviceTemplate},
		{Name: "httpScaledObjectTemplate", Config: httpScaledObjectTemplate},
	}

	if clusterScoped {
		templateData.HTTPInterceptorNamespace = KEDANamespace
		templateData.HTTPScalingSetKind = "ClusterHTTPScalingSet"
		templates = append(templates, Template{Name: "httpClusterScalingSetTemplate", Config: httpClusterScalingSetTemplate})
	} else {
		templates = append(templates, Template{Name: "httpScalingSetTemplate", Config: httpScalingSetTemplate})
		templates = append(templates, Template{Name: "httpScalingSetServiceAccount", Config: httpScalingSetServiceAccount})
		templates = append(templates, Template{Name: "httpScalingSetClusterRole", Config: httpScalingSetClusterRole})
		templates = append(templates, Template{Name: "httpScalingSetClusterRoleBinding", Config: httpScalingSetClusterRoleBinding})
	}

	return templateData, templates
}

func GetLoadJobTemplate() Template {
	return Template{Name: "loadJobTemplate", Config: loadJobTemplate}
}

func WaitForScalingSetComponents(t *testing.T, kc *kubernetes.Clientset, data TemplateData) {
	interceptorName := fmt.Sprintf("%s-interceptor", data.HTTPScalingSetName)
	scalerName := fmt.Sprintf("%s-external-scaler", data.HTTPScalingSetName)
	namespace := data.TestNamespace
	if data.ClusterScoped {
		namespace = KEDANamespace
	}

	// Wait for interceptor
	assert.True(t, WaitForDeploymentReplicaReadyCount(t, kc, interceptorName, namespace, 1, 6, 10),
		"replica count should be %d after 1 minutes", 1)

	// Wait for scaler
	assert.True(t, WaitForDeploymentReplicaReadyCount(t, kc, scalerName, namespace, 1, 6, 10),
		"replica count should be %d after 1 minutes", 1)
}
