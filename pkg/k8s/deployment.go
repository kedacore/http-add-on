package k8s

import (
	"context"
	"errors"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// DeleteDeployment deletes the deployment given using the client given
func DeleteDeployment(ctx context.Context, namespace, name string, cl client.Client) error {
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	if err := cl.Delete(ctx, deployment, &client.DeleteOptions{}); err != nil {
		return err
	}
	return nil
}

// NewDeployment creates a new deployment object
// with the given name and the given image. This does not actually create
// the deployment in the cluster, it just creates the deployment object
// in memory
func NewDeployment(
	namespace,
	name,
	image string,
	ports []int32,
	env []corev1.EnvVar,
	labels map[string]string,
	pullPolicy corev1.PullPolicy,
) *appsv1.Deployment {
	containerPorts := make([]corev1.ContainerPort, len(ports))
	for i, port := range ports {
		containerPorts[i] = corev1.ContainerPort{
			ContainerPort: port,
		}
	}
	deployment := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind: "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Replicas: Int32P(1),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Image:           image,
							Name:            name,
							ImagePullPolicy: pullPolicy,
							Ports:           containerPorts,
							Env:             env,
						},
					},
				},
			},
		},
	}

	return deployment
}

// AddLivenessProbe adds a liveness probe to the first container on depl.
// the probe will do an HTTP GET to path on port.
//
// returns a non-nil error if there is not at least one container on the given
// deployment's container list (depl.Spec.Template.Spec.Containers)
func AddLivenessProbe(
	depl *appsv1.Deployment,
	path string,
	port int,
) error {
	if len(depl.Spec.Template.Spec.Containers) < 1 {
		return errors.New("no conatiners to set liveness/readiness checks on")
	}
	depl.Spec.Template.Spec.Containers[0].LivenessProbe = &corev1.Probe{
		Handler: corev1.Handler{
			HTTPGet: &corev1.HTTPGetAction{
				Path: path,
				Port: intstr.FromInt(port),
			},
		},
		PeriodSeconds: 1,
	}
	return nil
}

// AddReadinessProbe adds a readiness probe to the first container on depl.
// the probe will do an HTTP GET to path on port.
//
// returns a non-nil error if there is not at least one container on the given
// deployment's container list (depl.Spec.Template.Spec.Containers)
func AddReadinessProbe(
	depl *appsv1.Deployment,
	path string,
	port int,
) error {
	depl.Spec.Template.Spec.Containers[0].ReadinessProbe = &corev1.Probe{
		Handler: corev1.Handler{
			HTTPGet: &corev1.HTTPGetAction{
				Path: path,
				Port: intstr.FromInt(port),
			},
		},
		PeriodSeconds: 1,
	}
	return nil
}

func ensureLeadingSlash(str string) string {
	if len(str) == 0 {
		return str
	}
	if str[0] != '/' {
		str = fmt.Sprintf("/%s", str)
	}
	return str
}

func AddHTTPLivenessProbe(depl *appsv1.Deployment, httpPath string, port int) {
	httpPath = ensureLeadingSlash(httpPath)
	depl.Spec.Template.Spec.Containers[0].LivenessProbe = &corev1.Probe{
		Handler: corev1.Handler{
			HTTPGet: &corev1.HTTPGetAction{
				Path: httpPath,
				Port: intstr.FromInt(port),
			},
		},
	}
}

func AddHTTPReadinessProbe(depl *appsv1.Deployment, httpPath string, port int) {
	httpPath = ensureLeadingSlash(httpPath)
	depl.Spec.Template.Spec.Containers[0].ReadinessProbe = &corev1.Probe{
		Handler: corev1.Handler{
			HTTPGet: &corev1.HTTPGetAction{
				Path: httpPath,
				Port: intstr.FromInt(port),
			},
		},
	}
}
