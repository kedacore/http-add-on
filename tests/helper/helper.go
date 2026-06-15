//go:build e2e

package helper

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"
	"text/template"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

const (
	KEDANamespace = "keda"
)

var (
	KubeClient *kubernetes.Clientset
	KubeConfig *rest.Config
)

type ExecutionError struct {
	StdError []byte
}

func (ee ExecutionError) Error() string {
	return string(ee.StdError)
}

func ParseCommand(cmdWithArgs string) *exec.Cmd {
	quoted := false
	splitCmd := strings.FieldsFunc(cmdWithArgs, func(r rune) bool {
		if r == '\'' {
			quoted = !quoted
		}
		return !quoted && r == ' '
	})
	for i, s := range splitCmd {
		if strings.HasPrefix(s, "'") && strings.HasSuffix(s, "'") {
			splitCmd[i] = s[1 : len(s)-1]
		}
	}

	return exec.Command(splitCmd[0], splitCmd[1:]...)
}

func ParseCommandWithDir(cmdWithArgs, dir string) *exec.Cmd {
	cmd := ParseCommand(cmdWithArgs)
	cmd.Dir = dir

	return cmd
}

func ExecuteCommand(cmdWithArgs string) ([]byte, error) {
	out, err := ParseCommand(cmdWithArgs).Output()
	if err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			return out, ExecutionError{StdError: exitError.Stderr}
		}
	}

	return out, err
}

func ExecuteCommandWithRetry(t *testing.T, cmdWithArgs string, retries int, delay time.Duration) ([]byte, error) {
	t.Helper()
	var out []byte
	var err error
	for i := 1; i <= retries; i++ {
		out, err = ExecuteCommand(cmdWithArgs)
		if err == nil {
			return out, nil
		}
		if i == retries {
			break
		}
		t.Logf("WARNING: command failed (attempt %d/%d), retrying in %v...: %v", i, retries, delay, err)
		time.Sleep(delay)
	}
	return out, err
}

func ExecuteCommandWithDir(cmdWithArgs, dir string) ([]byte, error) {
	out, err := ParseCommandWithDir(cmdWithArgs, dir).Output()
	if err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			return out, ExecutionError{StdError: exitError.Stderr}
		}
	}

	return out, err
}

func ExecCommandOnSpecificPod(t *testing.T, podName string, namespace string, command string) (string, string, error) {
	cmd := []string{
		"sh",
		"-c",
		command,
	}
	buf := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	request := KubeClient.CoreV1().RESTClient().Post().
		Resource("pods").Name(podName).Namespace(namespace).
		SubResource("exec").Timeout(time.Second*20).
		VersionedParams(&corev1.PodExecOptions{
			Command: cmd,
			Stdin:   false,
			Stdout:  true,
			Stderr:  true,
			TTY:     true,
		}, scheme.ParameterCodec)
	exec, err := remotecommand.NewSPDYExecutor(KubeConfig, "POST", request.URL())
	require.NoErrorf(t, err, "cannot execute command - %s", err)
	if err != nil {
		return "", "", err
	}
	err = exec.StreamWithContext(context.Background(), remotecommand.StreamOptions{
		Stdout: buf,
		Stderr: errBuf,
	})
	out := buf.String()
	errOut := errBuf.String()
	return out, errOut, err
}

func GetKubernetesClient(t *testing.T) *kubernetes.Clientset {
	if KubeClient != nil && KubeConfig != nil {
		return KubeClient
	}

	var err error
	KubeConfig, err = config.GetConfig()
	require.NoErrorf(t, err, "cannot fetch kube config file - %s", err)

	KubeClient, err = kubernetes.NewForConfig(KubeConfig)
	assert.NoErrorf(t, err, "cannot create kubernetes client - %s", err)

	return KubeClient
}

// Creates a new namespace. If it already exists, make sure it is deleted first.
func CreateNamespace(t *testing.T, kc *kubernetes.Clientset, nsName string) {
	DeleteNamespace(t, nsName)
	WaitForNamespaceDeletion(t, nsName)

	t.Logf("Creating namespace - %s", nsName)
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   nsName,
			Labels: map[string]string{"type": "e2e"},
		},
	}

	_, err := kc.CoreV1().Namespaces().Create(context.Background(), namespace, metav1.CreateOptions{})
	assert.NoErrorf(t, err, "cannot create kubernetes namespace - %s", err)
}

func DeleteNamespace(t *testing.T, nsName string) {
	t.Logf("deleting namespace %s", nsName)
	period := int64(0)
	kc := GetKubernetesClient(t)
	err := kc.CoreV1().Namespaces().Delete(context.Background(), nsName, metav1.DeleteOptions{
		GracePeriodSeconds: &period,
	})
	if apierrors.IsNotFound(err) {
		err = nil
	}
	assert.NoErrorf(t, err, "cannot delete kubernetes namespace - %s", err)
}

func WaitForJobSuccess(t *testing.T, kc *kubernetes.Clientset, jobName, namespace string, iterations, interval int) bool {
	for range iterations {
		job, err := kc.BatchV1().Jobs(namespace).Get(context.Background(), jobName, metav1.GetOptions{})
		if err != nil {
			t.Logf("cannot run job - %s", err)
		}

		if job.Status.Succeeded > 0 {
			t.Logf("job %s ran successfully!", jobName)
			return true // Job ran successfully
		}
		time.Sleep(time.Duration(interval) * time.Second)
	}
	return false
}

func WaitForNamespaceDeletion(t *testing.T, nsName string) bool {
	for range 120 {
		t.Logf("waiting for namespace %s deletion", nsName)
		_, err := KubeClient.CoreV1().Namespaces().Get(context.Background(), nsName, metav1.GetOptions{})
		if err != nil && apierrors.IsNotFound(err) {
			return true
		}
		time.Sleep(time.Second * 5)
	}
	return false
}

// Waits until deployment ready replica count hits target or number of iterations are done.
func WaitForDeploymentReplicaReadyCount(t *testing.T, kc *kubernetes.Clientset, name, namespace string,
	target, iterations, intervalSeconds int,
) bool {
	for range iterations {
		deployment, _ := kc.AppsV1().Deployments(namespace).Get(context.Background(), name, metav1.GetOptions{})
		replicas := deployment.Status.ReadyReplicas

		t.Logf("Waiting for deployment replicas to hit target. Deployment - %s, Current  - %d, Target - %d",
			name, replicas, target)

		if replicas == int32(target) {
			return true
		}

		time.Sleep(time.Duration(intervalSeconds) * time.Second)
	}

	return false
}

type Template struct {
	Name, Config string
}

func KubectlApplyWithTemplate(t *testing.T, data any, templateName string, config string) {
	t.Logf("Applying template: %s", templateName)

	tmpl, err := template.New("kubernetes resource template").Parse(config)
	require.NoErrorf(t, err, "cannot parse template - %s", err)

	tempFile, err := os.CreateTemp("", templateName)
	require.NoErrorf(t, err, "cannot create temp file - %s", err)

	defer os.Remove(tempFile.Name())

	err = tmpl.Execute(tempFile, data)
	require.NoErrorf(t, err, "cannot insert data into template - %s", err)

	_, err = ExecuteCommand(fmt.Sprintf("kubectl apply -f %s", tempFile.Name()))
	require.NoErrorf(t, err, "cannot apply file - %s", err)

	err = tempFile.Close()
	require.NoErrorf(t, err, "cannot close temp file - %s", err)
}

// Apply templates in order of slice
func KubectlApplyMultipleWithTemplate(t *testing.T, data any, templates []Template) {
	for _, tmpl := range templates {
		KubectlApplyWithTemplate(t, data, tmpl.Name, tmpl.Config)
	}
}

func KubectlDeleteWithTemplate(t *testing.T, data any, templateName, config string) {
	t.Logf("Deleting template: %s", templateName)

	tmpl, err := template.New("kubernetes resource template").Parse(config)
	require.NoErrorf(t, err, "cannot parse template - %s", err)

	tempFile, err := os.CreateTemp("", templateName)
	require.NoErrorf(t, err, "cannot delete temp file - %s", err)

	defer os.Remove(tempFile.Name())

	err = tmpl.Execute(tempFile, data)
	require.NoErrorf(t, err, "cannot insert data into template - %s", err)

	_, err = ExecuteCommand(fmt.Sprintf("kubectl delete -f %s", tempFile.Name()))
	require.NoErrorf(t, err, "cannot apply file - %s", err)

	err = tempFile.Close()
	assert.NoErrorf(t, err, "cannot close temp file - %s", err)
}

// Delete templates in reverse order of slice
func KubectlDeleteMultipleWithTemplate(t *testing.T, data any, templates []Template) {
	for _, tmpl := range slices.Backward(templates) {
		KubectlDeleteWithTemplate(t, data, tmpl.Name, tmpl.Config)
	}
}

func CreateKubernetesResources(t *testing.T, kc *kubernetes.Clientset, nsName string, data any, templates []Template) {
	CreateNamespace(t, kc, nsName)
	KubectlApplyMultipleWithTemplate(t, data, templates)
}

func DeleteKubernetesResources(t *testing.T, nsName string, data any, templates []Template) {
	KubectlDeleteMultipleWithTemplate(t, data, templates)
	DeleteNamespace(t, nsName)
	deleted := WaitForNamespaceDeletion(t, nsName)
	assert.Truef(t, deleted, "%s namespace not deleted", nsName)
}

func FindPodLogs(kc *kubernetes.Clientset, namespace, label string) ([]string, error) {
	var podLogs []string
	pods, err := kc.CoreV1().Pods(namespace).List(context.TODO(),
		metav1.ListOptions{LabelSelector: label})
	if err != nil {
		return []string{}, err
	}
	var podLogRequest *rest.Request
	for _, v := range pods.Items {
		podLogRequest = kc.CoreV1().Pods(namespace).GetLogs(v.Name, &corev1.PodLogOptions{})
		stream, err := podLogRequest.Stream(context.TODO())
		if err != nil {
			return []string{}, err
		}
		defer stream.Close()
		for {
			buf := make([]byte, 2000)
			numBytes, err := stream.Read(buf)
			if err == io.EOF {
				break
			}
			if numBytes == 0 {
				continue
			}
			if err != nil {
				return []string{}, err
			}
			podLogs = append(podLogs, string(buf[:numBytes]))
		}
	}
	return podLogs, nil
}

// KubectlGetResult runs `kubectl get` with parameters
func KubectlGetResult(t *testing.T, kind string, name string, namespace string, otherparameter string) string {
	time.Sleep(1 * time.Second) // wait a second for recource deployment finished
	kctlGetCmd := fmt.Sprintf(`kubectl get %s/%s -n %s %s"`, kind, name, namespace, otherparameter)
	t.Log("Running kubectl cmd:", kctlGetCmd)
	output, err := ExecuteCommand(kctlGetCmd)
	require.NoErrorf(t, err, "cannot get rollout info - %s", err)

	return strings.ReplaceAll(string(output), "\"", "")
}
