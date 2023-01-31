package k8s

import (
	"bytes"
	"text/template"

	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	soPollingInterval = 1
	soTriggerType     = "external-push"

	mkScalerAddress = "scalerAddress"
	mkHost          = "host"
)

// NewScaledObject creates a new ScaledObject in memory
func NewScaledObject(
	namespace string,
	name string,
	deploymentName string,
	scalerAddress string,
	host string,
	minReplicas,
	maxReplicas int32,
	idleReplicas int32,
	cooldownPeriod int32,
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
		"IdleReplicas":   idleReplicas,
		"DeploymentName": deploymentName,
		"ScalerAddress":  scalerAddress,
		"Host":           host,
		"CooldownPeriod": cooldownPeriod,
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
