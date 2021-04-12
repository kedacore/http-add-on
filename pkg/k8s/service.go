package k8s

import (
	context "context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	k8scorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

func NewTCPServicePort(name string, port int32, targetPort int32) corev1.ServicePort {
	return corev1.ServicePort{
		Name:     name,
		Protocol: corev1.ProtocolTCP,
		Port:     port,
		TargetPort: intstr.IntOrString{
			Type:   intstr.Int,
			IntVal: targetPort,
		},
	}
}

func DeleteService(ctx context.Context, name string, cl k8scorev1.ServiceInterface) error {
	return cl.Delete(ctx, name, metav1.DeleteOptions{})
}

// NewService creates a new Service object in memory according to the input parameters.
// This function operates in memory only and doesn't do any I/O whatsoever.
func NewService(
	namespace,
	name string,
	servicePorts []corev1.ServicePort,
	svcType corev1.ServiceType,
	selector map[string]string,
) *corev1.Service {
	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind: "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
			Labels:    selector,
		},
		Spec: corev1.ServiceSpec{
			Ports:    servicePorts,
			Selector: selector,
			Type:     svcType,
		},
	}
}
