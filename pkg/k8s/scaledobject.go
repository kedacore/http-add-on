package k8s

import (
	"bytes"
	"context"
	"embed"
	"text/template"

	unstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

//go:embed templates
var scaledObjectTemplateFS embed.FS

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
	scalerAddress,
	host string,
	minReplicas,
	maxReplicas int32,
) (*unstructured.Unstructured, error) {
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

	tpl, err := template.ParseFS(scaledObjectTemplateFS, "templates/scaledobject.yaml")
	if err != nil {
		return nil, err
	}

	var scaledObjectTemplateBuffer bytes.Buffer
	if tplErr := tpl.Execute(&scaledObjectTemplateBuffer, map[string]interface{}{
		"Name":           name,
		"Namespace":      namespace,
		"Labels":         labels,
		"MinReplicas":    minReplicas,
		"MaxReplicas":    maxReplicas,
		"DeploymentName": deploymentName,
		"ScalerAddress":  scalerAddress,
		"Host":           host,
	}); tplErr != nil {
		return nil, tplErr
	}

	var decodedYaml map[string]interface{}
	decodeErr := yaml.Unmarshal(scaledObjectTemplateBuffer.Bytes(), &decodedYaml)
	if decodeErr != nil {
		return nil, decodeErr
	}

	return &unstructured.Unstructured{
		Object: decodedYaml,
	}, nil
}
