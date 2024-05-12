package k8s

import (
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func NewService(name, namespace, portName string, port int32, selector map[string]string) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: v1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name: portName,
					Port: port,
					TargetPort: intstr.IntOrString{
						StrVal: portName,
						Type:   intstr.String,
					},
					Protocol: "TCP",
				},
			},
			Type:     corev1.ServiceTypeClusterIP,
			Selector: selector,
		},
	}
}
