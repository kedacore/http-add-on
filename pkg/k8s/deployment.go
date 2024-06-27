package k8s

import (
	"golang.org/x/exp/maps"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func NewDeployment(name, namespace, serviceAccount, image string, ports []corev1.ContainerPort, envs []corev1.EnvVar, replicas *int32, selector, labels, anntotaions map[string]string, resources corev1.ResourceRequirements) *appsv1.Deployment {
	if labels == nil {
		labels = map[string]string{}
	}
	if anntotaions == nil {
		anntotaions = map[string]string{}
	}
	maps.Copy(labels, selector)
	return &appsv1.Deployment{
		ObjectMeta: v1.ObjectMeta{
			Name:        name,
			Namespace:   namespace,
			Labels:      labels,
			Annotations: anntotaions,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: replicas,
			Selector: &v1.LabelSelector{
				MatchLabels: selector,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: v1.ObjectMeta{
					Labels:      labels,
					Annotations: anntotaions,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: serviceAccount,
					Containers: []corev1.Container{
						{
							Name:      name,
							Image:     image,
							Ports:     ports,
							Resources: resources,
							Env:       envs,
						},
					},
				},
			},
		},
	}
}
