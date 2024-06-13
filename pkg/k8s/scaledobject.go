package k8s

import (
	kedav1alpha1 "github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"github.com/kedacore/http-add-on/operator/apis/http/v1alpha1"
)

const (
	soPollingInterval = 15
	soTriggerType     = "external-push"

	ScalerAddressKey           = "scalerAddress"
	HTTPScaledObjectKey        = "httpScaledObject"
	HTTPScaledObjectVersionKey = "httpScaledObjectVersion"
)

// NewScaledObject creates a new ScaledObject in memory
func NewScaledObject(namespace string, name string, labels map[string]string, annotations map[string]string,
	workloadRef v1alpha1.ScaleTargetRef, scalerAddress string, minReplicas *int32, maxReplicas *int32, cooldownPeriod *int32, httpSoVersion string) *kedav1alpha1.ScaledObject {
	return &kedav1alpha1.ScaledObject{
		TypeMeta: metav1.TypeMeta{
			APIVersion: kedav1alpha1.SchemeGroupVersion.Identifier(),
			Kind:       ObjectKind(&kedav1alpha1.ScaledObject{}),
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   namespace,
			Name:        name,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: kedav1alpha1.ScaledObjectSpec{
			ScaleTargetRef: &kedav1alpha1.ScaleTarget{
				APIVersion: workloadRef.APIVersion,
				Kind:       workloadRef.Kind,
				Name:       workloadRef.Name,
			},
			PollingInterval: ptr.To[int32](soPollingInterval),
			CooldownPeriod:  cooldownPeriod,
			MinReplicaCount: minReplicas,
			MaxReplicaCount: maxReplicas,
			Advanced: &kedav1alpha1.AdvancedConfig{
				RestoreToOriginalReplicaCount: true,
			},
			Triggers: []kedav1alpha1.ScaleTriggers{
				{
					Type: soTriggerType,
					Metadata: map[string]string{
						ScalerAddressKey:    scalerAddress,
						HTTPScaledObjectKey: name,
						// Store HTTPSO resource version in the metadata, this ensures that the ScaledObject is reconciled when the HTTPSO is updated
						HTTPScaledObjectVersionKey: httpSoVersion,
					},
				},
			},
		},
	}
}
