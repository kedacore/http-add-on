package k8s

import (
	"strings"

	kedav1alpha1 "github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

const (
	soPollingInterval = 1
	soTriggerType     = "external-push"

	mkScalerAddress = "scalerAddress"
	mkHosts         = "hosts"
	mkPathPrefixes  = "pathPrefixes"
)

// NewScaledObject creates a new ScaledObject in memory
func NewScaledObject(
	namespace string,
	name string,
	deploymentName string,
	scalerAddress string,
	hosts []string,
	pathPrefixes []string,
	minReplicas *int32,
	maxReplicas *int32,
	cooldownPeriod *int32,
) *kedav1alpha1.ScaledObject {
	return &kedav1alpha1.ScaledObject{
		TypeMeta: metav1.TypeMeta{
			APIVersion: kedav1alpha1.SchemeGroupVersion.Identifier(),
			Kind:       ObjectKind(&kedav1alpha1.ScaledObject{}),
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Spec: kedav1alpha1.ScaledObjectSpec{
			ScaleTargetRef: &kedav1alpha1.ScaleTarget{
				APIVersion: appsv1.SchemeGroupVersion.Identifier(),
				Kind:       ObjectKind(&appsv1.Deployment{}),
				Name:       deploymentName,
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
						mkScalerAddress: scalerAddress,
						mkHosts:         strings.Join(hosts, ","),
						mkPathPrefixes:  strings.Join(pathPrefixes, ","),
					},
				},
			},
		},
	}
}
