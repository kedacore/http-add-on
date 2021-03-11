package k8s

import (
	"context"
	keda "github.com/kedacore/keda/v2/api/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// DeleteScaledObject deletes a scaled object with the given name
func DeleteScaledObject(ctx context.Context, name string, namespace string, cl client.Client) error {
	scaledObj := &keda.ScaledObject{
		TypeMeta: v1.TypeMeta{
			APIVersion: "keda.sh/v1alpha1",
			Kind:       "ScaledObject",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}

	if err := cl.Delete(ctx, scaledObj, &client.DeleteOptions{}); err != nil {
		return err
	}
	return nil
}

// NewScaledObject creates a new ScaledObject in memory
func NewScaledObject(
	namespace,
	name,
	deploymentName,
	scalerAddress string,
) *keda.ScaledObject {
	return &keda.ScaledObject{
		TypeMeta: v1.TypeMeta{
			Kind:       "ScaledObject",
			APIVersion: "keda.sh/v1alpha1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    Labels(name),
		},
		Spec: keda.ScaledObjectSpec{
			PollingInterval: int32P(250),
			ScaleTargetRef: &keda.ScaleTarget{
				Kind: "Deployment",
				Name: deploymentName,
			},
			Triggers: []keda.ScaleTriggers{
				{
					Type: "external",
					Metadata: map[string]string{
						"scalerAddress": scalerAddress,
					},
				},
			},
		},
	}
}
