package k8s

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sappsv1 "k8s.io/client-go/kubernetes/typed/apps/v1"
)

// DeleteDeployment deletes the deployment given using the client given
func DeleteDeployment(ctx context.Context, name string, cl k8sappsv1.DeploymentInterface) error {
	return cl.Delete(ctx, name, metav1.DeleteOptions{})
}

// NewDeployment creates a new deployment object
// with the given name and the given image. This does not actually create
// the deployment in the cluster, it just creates the deployment object
// in memory
func NewDeployment(
	name,
	image string,
	ports []int32,
	env []corev1.EnvVar,
	labels map[string]string,
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
			Name:   name,
			Labels: labels,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Replicas: int32P(1),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Image:           image,
							Name:            name,
							ImagePullPolicy: "Always",
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
