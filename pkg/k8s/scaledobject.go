package k8s

import (
	"context"
	"sigs.k8s.io/controller-runtime/pkg/client"

	unstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func kedaGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    "keda.sh",
		Version:  "v1alpha1",
		Resource: "scaledobjects",
	}
}

// DeleteScaledObject deletes a scaled object with the given name
func DeleteScaledObject(ctx context.Context, name string, namespace string, cl client.Client) error {
	scaledObj := &unstructured.Unstructured{}
	scaledObj.SetName(name)
	scaledObj.SetNamespace(namespace)
	scaledObj.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "keda.sh/v1alpha1",
		Kind:    "ScaledObject",
		Version: "v1alpha1",
	})

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
) *unstructured.Unstructured {
	// https://keda.sh/docs/1.5/faq/
	// https://github.com/kedacore/keda/blob/aa0ea79450a1c7549133aab46f5b916efa2364ab/api/v1alpha1/scaledobject_types.go
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "keda.sh/v1alpha1",
			"kind":       "ScaledObject",
			"metadata": map[string]interface{}{
				"namespace": namespace,
				"name":      name,
				"labels":    Labels(name),
			},
			"spec": map[string]interface{}{
				"minReplicaCount": 0,
				"maxReplicaCount": 1000,
				"pollingInterval": 250,
				"scaleTargetRef": map[string]string{
					"name": deploymentName,
					// "apiVersion": "apps/v1",
					"kind": "Deployment",
				},
				"triggers": []interface{}{
					map[string]interface{}{
						"type": "external",
						"metadata": map[string]string{
							"scalerAddress": scalerAddress,
						},
					},
				},
			},
		},
	}
}
