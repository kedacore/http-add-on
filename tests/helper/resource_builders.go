//go:build e2e || stress
// +build e2e stress

package helper

import (
	"bytes"
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// ServiceSpec defines a Kubernetes Service configuration
type ServiceSpec struct {
	Name       string
	Namespace  string
	App        string
	Port       int32
	TargetPort string
}

// BuildServiceYAML generates YAML for a Kubernetes Service
func BuildServiceYAML(s ServiceSpec) string {
	svc := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Service",
		"metadata": map[string]interface{}{
			"name":      s.Name,
			"namespace": s.Namespace,
			"labels": map[string]string{
				"app": s.App,
			},
		},
		"spec": map[string]interface{}{
			"ports": []map[string]interface{}{
				{
					"port":       s.Port,
					"targetPort": s.TargetPort,
					"protocol":   "TCP",
					"name":       "http",
				},
			},
			"selector": map[string]string{
				"app": s.App,
			},
		},
	}
	return toYAML(svc)
}

// DeploymentSpec defines a Kubernetes Deployment configuration
type DeploymentSpec struct {
	Name          string
	Namespace     string
	App           string
	Replicas      int32
	Image         string
	Args          []string
	ContainerPort int32
	ReadinessPath string
	ReadinessPort string
}

// BuildDeploymentYAML generates YAML for a Kubernetes Deployment
func BuildDeploymentYAML(d DeploymentSpec) string {
	container := map[string]interface{}{
		"name":  d.App,
		"image": d.Image,
		"ports": []map[string]interface{}{
			{
				"name":          "http",
				"containerPort": d.ContainerPort,
				"protocol":      "TCP",
			},
		},
	}

	if len(d.Args) > 0 {
		container["args"] = d.Args
	}

	if d.ReadinessPath != "" {
		container["readinessProbe"] = map[string]interface{}{
			"httpGet": map[string]interface{}{
				"path": d.ReadinessPath,
				"port": d.ReadinessPort,
			},
		}
	}

	deployment := map[string]interface{}{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata": map[string]interface{}{
			"name":      d.Name,
			"namespace": d.Namespace,
			"labels": map[string]string{
				"app": d.App,
			},
		},
		"spec": map[string]interface{}{
			"replicas": d.Replicas,
			"selector": map[string]interface{}{
				"matchLabels": map[string]string{
					"app": d.App,
				},
			},
			"template": map[string]interface{}{
				"metadata": map[string]interface{}{
					"labels": map[string]string{
						"app": d.App,
					},
				},
				"spec": map[string]interface{}{
					"containers": []interface{}{container},
				},
			},
		},
	}
	return toYAML(deployment)
}

// JobSpec defines a Kubernetes Job configuration
type JobSpec struct {
	Name                       string
	Namespace                  string
	Image                      string
	Args                       []string
	RestartPolicy              string
	TerminationGracePeriodSecs int64
	ActiveDeadlineSeconds      int64
	BackoffLimit               int32
}

// BuildJobYAML generates YAML for a Kubernetes Job
func BuildJobYAML(j JobSpec) string {
	restartPolicy := j.RestartPolicy
	if restartPolicy == "" {
		restartPolicy = "Never"
	}

	job := map[string]interface{}{
		"apiVersion": "batch/v1",
		"kind":       "Job",
		"metadata": map[string]interface{}{
			"name":      j.Name,
			"namespace": j.Namespace,
		},
		"spec": map[string]interface{}{
			"template": map[string]interface{}{
				"spec": map[string]interface{}{
					"containers": []map[string]interface{}{
						{
							"name":            j.Name,
							"image":           j.Image,
							"imagePullPolicy": "Always",
							"args":            j.Args,
						},
					},
					"restartPolicy":                 restartPolicy,
					"terminationGracePeriodSeconds": j.TerminationGracePeriodSecs,
				},
			},
			"activeDeadlineSeconds": j.ActiveDeadlineSeconds,
			"backoffLimit":          j.BackoffLimit,
		},
	}
	return toYAML(job)
}

// OhaLoadJobSpec defines a specialized Job for oha load generator
type OhaLoadJobSpec struct {
	Name                       string
	Namespace                  string
	Host                       string // Target host header
	TargetURL                  string // Target URL (default: http://keda-add-ons-http-interceptor-proxy.keda:8080/)
	Requests                   int    // Number of requests (-n). If 0, uses Duration instead
	Duration                   string // Duration (-z), e.g. "60s", "10m". Used if Requests is 0
	Concurrency                int    // Concurrent connections (-c)
	RateLimit                  int    // Rate limit per second (-q). 0 means no limit
	TerminationGracePeriodSecs int64
	ActiveDeadlineSeconds      int64
	BackoffLimit               int32
}

// BuildOhaLoadJobYAML generates YAML for an oha load generator Job
func BuildOhaLoadJobYAML(j OhaLoadJobSpec) string {
	targetURL := j.TargetURL
	if targetURL == "" {
		targetURL = "http://keda-add-ons-http-interceptor-proxy.keda:8080/"
	}

	args := []string{"--no-tui"}

	// Request count or duration
	if j.Requests > 0 {
		args = append(args, "-n", fmt.Sprintf("%d", j.Requests))
	} else if j.Duration != "" {
		args = append(args, "-z", j.Duration)
	}

	// Concurrency
	if j.Concurrency > 0 {
		args = append(args, "-c", fmt.Sprintf("%d", j.Concurrency))
	}

	// Rate limit
	if j.RateLimit > 0 {
		args = append(args, "-q", fmt.Sprintf("%d", j.RateLimit))
	}

	// Host header
	args = append(args, "-H", "Host: "+j.Host)

	// Target URL
	args = append(args, targetURL)

	job := map[string]interface{}{
		"apiVersion": "batch/v1",
		"kind":       "Job",
		"metadata": map[string]interface{}{
			"name":      j.Name,
			"namespace": j.Namespace,
		},
		"spec": map[string]interface{}{
			"template": map[string]interface{}{
				"spec": map[string]interface{}{
					"containers": []map[string]interface{}{
						{
							"name":            j.Name,
							"image":           "ghcr.io/hatoo/oha:1.13",
							"imagePullPolicy": "Always",
							"args":            args,
						},
					},
					"restartPolicy":                 "Never",
					"terminationGracePeriodSeconds": j.TerminationGracePeriodSecs,
				},
			},
			"activeDeadlineSeconds": j.ActiveDeadlineSeconds,
			"backoffLimit":          j.BackoffLimit,
		},
	}
	return toYAML(job)
}

// HTTPScaledObjectSpec defines an HTTPScaledObject configuration
type HTTPScaledObjectSpec struct {
	Name            string
	Namespace       string
	Hosts           []string
	DeploymentName  string
	ServiceName     string
	Port            int32
	MinReplicas     int32
	MaxReplicas     int32
	ScaledownPeriod int32
	TargetValue     int
	RateWindow      time.Duration
	RateGranularity time.Duration
}

// BuildHTTPScaledObjectYAML generates YAML for an HTTPScaledObject
func BuildHTTPScaledObjectYAML(h HTTPScaledObjectSpec) string {
	spec := map[string]interface{}{
		"hosts": h.Hosts,
		"scaleTargetRef": map[string]interface{}{
			"name":    h.DeploymentName,
			"service": h.ServiceName,
			"port":    h.Port,
		},
		"replicas": map[string]interface{}{
			"min": h.MinReplicas,
			"max": h.MaxReplicas,
		},
	}

	if h.ScaledownPeriod > 0 {
		spec["scaledownPeriod"] = h.ScaledownPeriod
	}

	if h.TargetValue > 0 {
		scalingMetric := map[string]interface{}{
			"requestRate": map[string]interface{}{
				"targetValue": h.TargetValue,
			},
		}
		if h.RateWindow > 0 {
			scalingMetric["requestRate"].(map[string]interface{})["window"] = h.RateWindow.String()
		}
		if h.RateGranularity > 0 {
			scalingMetric["requestRate"].(map[string]interface{})["granularity"] = h.RateGranularity.String()
		}
		spec["scalingMetric"] = scalingMetric
	}

	httpso := map[string]interface{}{
		"apiVersion": "http.keda.sh/v1alpha1",
		"kind":       "HTTPScaledObject",
		"metadata": map[string]interface{}{
			"name":      h.Name,
			"namespace": h.Namespace,
		},
		"spec": spec,
	}
	return toYAML(httpso)
}

func toYAML(v interface{}) string {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	_ = enc.Encode(v)
	return buf.String()
}

// KubectlApplyYAML applies a raw YAML string directly
func KubectlApplyYAML(t interface {
	Logf(string, ...interface{})
	Helper()
}, name, yamlContent string) error {
	t.Helper()
	t.Logf("Applying resource: %s", name)

	tempFile, err := os.CreateTemp("", name+"-*.yaml")
	if err != nil {
		return err
	}
	defer os.Remove(tempFile.Name())

	if _, err := tempFile.WriteString(yamlContent); err != nil {
		return err
	}
	if err := tempFile.Close(); err != nil {
		return err
	}

	_, err = ExecuteCommand("kubectl apply -f " + tempFile.Name())
	return err
}

// KubectlDeleteYAML deletes resources defined in a raw YAML string
func KubectlDeleteYAML(t interface {
	Logf(string, ...interface{})
	Helper()
}, name, yamlContent string) error {
	t.Helper()
	t.Logf("Deleting resource: %s", name)

	tempFile, err := os.CreateTemp("", name+"-*.yaml")
	if err != nil {
		return err
	}
	defer os.Remove(tempFile.Name())

	if _, err := tempFile.WriteString(yamlContent); err != nil {
		return err
	}
	if err := tempFile.Close(); err != nil {
		return err
	}

	_, _ = ExecuteCommand("kubectl delete -f " + tempFile.Name())
	return nil
}
