package k8s

import (
	"context"
	"errors"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// newConfigMap creates a new configMap structure
func NewConfigMap(
	namespace string,
	name string,
	labels map[string]string,
	data map[string]string,
) *corev1.ConfigMap {

	configMap := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind: "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Data: data,
	}

	return configMap
}

func CreateConfigMap (
	ctx context.Context,
	cl client.Client,
	configMap *corev1.ConfigMap,
	logger logr.Logger,
) error {
	 existentConfigMap, err := GetConfigMap(ctx, cl, configMap.Namespace, configMap.Name)
	if err == nil { return err }
	if existentConfigMap.Name != "" { return errors.New("ConfigMap already exists") }

	createErr := cl.Create(ctx, configMap)
	if createErr != nil { return createErr }
	return nil
}

func DeleteConfigMap (
	ctx context.Context,
	cl client.Client,
	configMap *corev1.ConfigMap,
	logger logr.Logger,
) error {
	err := cl.Delete(ctx, configMap)
	if err != nil { return err }
	return nil
}

func PatchConfigMap (
	ctx context.Context,
	cl client.Client,
	logger logr.Logger,
	originalConfigMap *corev1.ConfigMap,
	patchConfigMap *corev1.ConfigMap,
) (*corev1.ConfigMap,error) {
	patchErr := cl.Patch(ctx, patchConfigMap, client.MergeFrom(originalConfigMap))
	if patchErr != nil { return nil, patchErr }
	return patchConfigMap, nil
}

func GetConfigMap (
	ctx context.Context,
	cl client.Client,
	namespace string,
	name string,
) (*corev1.ConfigMap, error) {

	configMap := &corev1.ConfigMap{}
	err := cl.Get(ctx, client.ObjectKey{ Name: name, Namespace: namespace }, configMap)
	if err != nil {
		return nil, err
	}
	return configMap, nil
}

