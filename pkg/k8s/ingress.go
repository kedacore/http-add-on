package k8s

import (
	v1beta1 "k8s.io/api/networking/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// NewIngress creates a new Ingress in the given namespace with the given name. It will have
// 1 rule in which the given host directs to the service with the given name at the given port.
// it will apply that rule to all paths
func NewIngress(namespace, name, host, svcName string, svcPort int32) *v1beta1.Ingress {
	return &v1beta1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Spec: v1beta1.IngressSpec{
			Rules: []v1beta1.IngressRule{
				{
					Host: host,
					IngressRuleValue: v1beta1.IngressRuleValue{
						HTTP: &v1beta1.HTTPIngressRuleValue{
							Paths: []v1beta1.HTTPIngressPath{
								{
									Backend: v1beta1.IngressBackend{
										ServiceName: svcName,
										ServicePort: intstr.IntOrString{
											Type:   intstr.Int,
											IntVal: svcPort,
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}
