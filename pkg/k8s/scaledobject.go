package k8s

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	unstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

func kedaGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    "keda.sh",
		Version:  "v1alpha1",
		Resource: "scaledobjects",
	}
}

// NewScaledObjectClient returns a new dynamic client capable
// of interacting with ScaledObjects in a cluster
func NewScaledObjectClient(cl dynamic.Interface) dynamic.NamespaceableResourceInterface {
	return cl.Resource(kedaGVR())
}

// DeleteScaledObject deletes a scaled object with the given name
func DeleteScaledObject(ctx context.Context, name string, cl dynamic.ResourceInterface) error {
	return cl.Delete(name, &metav1.DeleteOptions{})
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
	//
	// unstructured.Unstructured only supports specific types in it. see here for the list:
	// https://github.com/kubernetes/apimachinery/blob/v0.17.12/pkg/runtime/converter.go#L449-L476
	typedLabels := Labels(name)
	labels := map[string]interface{}{}
	for k, v := range typedLabels {
		var vIface interface{} = v
		labels[k] = vIface
	}
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "keda.sh/v1alpha1",
			"kind":       "ScaledObject",
			"metadata": map[string]interface{}{
				"namespace": namespace,
				"name":      name,
				"labels":    labels,
			},
			"spec": map[string]interface{}{
				"minReplicaCount": int64(0),
				"maxReplicaCount": int64(1000),
				"pollingInterval": int64(250),
				"scaleTargetRef": map[string]interface{}{
					"name": deploymentName,
					// "apiVersion": "apps/v1",
					"kind": "Deployment",
				},
				"triggers": []interface{}{
					map[string]interface{}{
						"type": "external",
						"metadata": map[string]interface{}{
							"scalerAddress": scalerAddress,
						},
					},
				},
			},
		},
	}
}
