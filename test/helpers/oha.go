//go:build e2e

package helpers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
)

const (
	// renovate: datasource=docker
	ohaImage = "ghcr.io/hatoo/oha:1.14.0"
)

// OhaResult maps the JSON output of oha.
type OhaResult struct {
	Summary            OhaSummary     `json:"summary"`
	LatencyPercentiles OhaPercentiles `json:"latencyPercentiles"`
	RPS                OhaRPSStats    `json:"rps"`
	StatusCodeDist     map[string]int `json:"statusCodeDistribution"`
	ErrorDist          map[string]int `json:"errorDistribution"`
}

// OhaSummary contains top-level request statistics.
type OhaSummary struct {
	SuccessRate    float64 `json:"successRate"`
	Total          float64 `json:"total"`
	Slowest        float64 `json:"slowest"`
	Fastest        float64 `json:"fastest"`
	Average        float64 `json:"average"`
	RequestsPerSec float64 `json:"requestsPerSec"`
	TotalData      int     `json:"totalData"`
	SizePerRequest int     `json:"sizePerRequest"`
}

// OhaPercentiles contains percentile distribution values.
type OhaPercentiles struct {
	P10   float64 `json:"p10"`
	P25   float64 `json:"p25"`
	P50   float64 `json:"p50"`
	P75   float64 `json:"p75"`
	P90   float64 `json:"p90"`
	P95   float64 `json:"p95"`
	P99   float64 `json:"p99"`
	P999  float64 `json:"p99.9"`
	P9999 float64 `json:"p99.99"`
}

// OhaRPSStats contains requests-per-second statistics.
type OhaRPSStats struct {
	Mean        float64        `json:"mean"`
	Stddev      float64        `json:"stddev"`
	Max         float64        `json:"max"`
	Percentiles OhaPercentiles `json:"percentiles"`
}

// OhaOpts configures an oha load test run.
type OhaOpts struct {
	Rate     int
	Duration time.Duration
	Host     string
}

// ErrorRate returns the fraction of failed requests (0.0 to 1.0).
func (r OhaResult) ErrorRate() float64 {
	return 1.0 - r.Summary.SuccessRate
}

// TotalRequests returns the total number of requests from status code distribution.
func (r OhaResult) TotalRequests() int {
	total := 0
	for _, count := range r.StatusCodeDist {
		total += count
	}
	return total
}

// RunOha creates a Pod running oha against the interceptor proxy service
// inside the cluster, waits for completion, and returns the parsed results.
func (f *Framework) RunOha(opts OhaOpts) OhaResult {
	f.t.Helper()

	targetURL := fmt.Sprintf("http://%s.%s.svc.cluster.local:%d/",
		interceptorProxyService, AddonNamespace, interceptorProxyPort)

	args := []string{
		"--no-tui",
		"--output-format", "json",
		"-z", opts.Duration.String(),
		"-H", "Host: " + opts.Host,
	}
	if opts.Rate > 0 {
		args = append(args, "-q", fmt.Sprintf("%d", opts.Rate))
	}
	args = append(args, targetURL)

	podName := randomName("oha")

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: f.namespace,
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			Containers: []corev1.Container{{
				Name:  "oha",
				Image: ohaImage,
				Args:  args,
			}},
		},
	}

	f.createResource(pod)

	waitTimeout := opts.Duration + 2*time.Minute
	if err := wait.For(
		conditions.New(f.client.Resources()).PodPhaseMatch(pod, corev1.PodSucceeded),
		wait.WithTimeout(waitTimeout),
	); err != nil {
		logs := f.podLogs(f.namespace, podName, "oha")
		f.t.Fatalf("oha pod did not succeed within %v: %v\nlogs:\n%s", waitTimeout, err, logs)
	}

	logs := f.podLogs(f.namespace, podName, "oha")

	var result OhaResult
	if err := json.Unmarshal([]byte(logs), &result); err != nil {
		f.t.Fatalf("failed to parse oha JSON output: %v\nraw output:\n%s", err, logs)
	}

	if result.TotalRequests() == 0 {
		f.t.Fatalf("oha completed zero requests; errors: %v\nraw output:\n%s", result.ErrorDist, logs)
	}

	return result
}

// LogOhaResult logs the oha results in a human-readable format.
func (f *Framework) LogOhaResult(r OhaResult) {
	f.t.Helper()
	f.t.Logf("requests=%d rate=%.1f/s success=%.1f%%",
		r.TotalRequests(), r.Summary.RequestsPerSec, r.Summary.SuccessRate*100)
	f.t.Logf("latency avg=%.3fs p50=%.3fs p95=%.3fs p99=%.3fs max=%.3fs",
		r.Summary.Average,
		r.LatencyPercentiles.P50,
		r.LatencyPercentiles.P95,
		r.LatencyPercentiles.P99,
		r.Summary.Slowest)
	f.t.Logf("status_codes=%v", r.StatusCodeDist)
	if len(r.ErrorDist) > 0 {
		f.t.Logf("errors=%v", r.ErrorDist)
	}
}

func (f *Framework) podLogs(namespace, podName, containerName string) string {
	f.t.Helper()

	clientset, err := kubernetes.NewForConfig(f.client.RESTConfig())
	if err != nil {
		f.t.Fatalf("failed to create clientset: %v", err)
	}

	req := clientset.CoreV1().Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{
		Container: containerName,
	})
	stream, err := req.Stream(f.ctx)
	if err != nil {
		f.t.Fatalf("failed to get logs for %s/%s/%s: %v", namespace, podName, containerName, err)
	}
	defer func() { _ = stream.Close() }()

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, stream); err != nil {
		f.t.Fatalf("failed to read logs for %s/%s/%s: %v", namespace, podName, containerName, err)
	}

	return buf.String()
}
