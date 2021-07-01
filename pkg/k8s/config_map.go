package k8s

import (
	"context"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ConfigMapGetter is a pared down version of a ConfigMapInterface
// (found here: https://pkg.go.dev/k8s.io/client-go@v0.21.3/kubernetes/typed/core/v1#ConfigMapInterface).
//
// Pass this whenever possible to functions that only need to get individual ConfigMaps
// from Kubernetes, and nothing else.
type ConfigMapGetter interface {
	Get(ctx context.Context, name string, opts metav1.GetOptions) (*corev1.ConfigMap, error)
}

// ConfigMapWatcher is a pared down version of a ConfigMapInterface
// (found here: https://pkg.go.dev/k8s.io/client-go@v0.21.3/kubernetes/typed/core/v1#ConfigMapInterface).
//
// Pass this whenever possible to functions that only need to watch for ConfigMaps
// from Kubernetes, and nothing else.
type ConfigMapWatcher interface {
	Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error)
}

// ConfigMapGetterWatcher is a pared down version of a ConfigMapInterface
// (found here: https://pkg.go.dev/k8s.io/client-go@v0.21.3/kubernetes/typed/core/v1#ConfigMapInterface).
//
// Pass this whenever possible to functions that only need to watch for ConfigMaps
// from Kubernetes, and nothing else.
type ConfigMapGetterWatcher interface {
	ConfigMapGetter
	ConfigMapWatcher
}

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

// CreateConfigMap sends a request to Kubernetes using the client cl
// to create configMap. Returns a non-nil error if anything failed with the creation,
// including if the config map already existed.
func CreateConfigMap(
	ctx context.Context,
	logger logr.Logger,
	cl client.Writer,
	configMap *corev1.ConfigMap,
) error {
	logger = logger.WithName("pkg.k8s.CreateConfigMap")
	if err := cl.Create(ctx, configMap); err != nil {
		logger.Error(
			err,
			"failed to create ConfigMap",
			"configMap",
			*configMap,
		)
		return err
	}
	return nil
}

func DeleteConfigMap(
	ctx context.Context,
	cl client.Writer,
	configMap *corev1.ConfigMap,
	logger logr.Logger,
) error {
	logger = logger.WithName("pkg.k8s.DeleteConfigMap")
	err := cl.Delete(ctx, configMap)
	if err != nil {
		logger.Error(
			err,
			"failed to delete configmap",
			"configMap",
			*configMap,
		)
		return err
	}
	return nil
}

func PatchConfigMap(
	ctx context.Context,
	logger logr.Logger,
	cl client.Writer,
	originalConfigMap *corev1.ConfigMap,
	patchConfigMap *corev1.ConfigMap,
) (*corev1.ConfigMap, error) {
	logger = logger.WithName("pkg.k8s.PatchConfigMap")
	if err := cl.Patch(
		ctx,
		patchConfigMap,
		client.MergeFrom(originalConfigMap),
	); err != nil {
		logger.Error(
			err,
			"failed to patch ConfigMap",
			"originalConfigMap",
			*originalConfigMap,
			"patchConfigMap",
			*patchConfigMap,
		)
		return nil, err
	}
	return patchConfigMap, nil
}

func GetConfigMap(
	ctx context.Context,
	cl client.Client,
	namespace string,
	name string,
) (*corev1.ConfigMap, error) {

	configMap := &corev1.ConfigMap{}
	err := cl.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, configMap)
	if err != nil {
		return nil, err
	}
	return configMap, nil
}
