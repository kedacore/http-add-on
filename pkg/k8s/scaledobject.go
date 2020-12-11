package k8s

import (
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	unstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

func kedaGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    "keda.k8s.io",
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
func DeleteScaledObject(name string, cl dynamic.ResourceInterface) error {
	return cl.Delete(name, &v1.DeleteOptions{})
}

// NewScaledObject creates a new ScaledObject in memory
func NewScaledObject(namespace, name, deploymentName, scalerAddress string) *unstructured.Unstructured {
	// https://keda.sh/docs/1.5/faq/
	// https://github.com/kedacore/keda/blob/v2/api/v1alpha1/scaledobject_types.go
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "keda.k8s.io/v1alpha1",
			"kind":       "ScaledObject",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
				"labels":    labels(name),
			},
			"spec": map[string]interface{}{
				"minReplicaCount": 0,
				"maxReplicaCount": 1000,
				"pollingInterval": 1,
				"scaleTargetRef": map[string]string{
					"deploymentName": deploymentName,
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
