package k8s

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type FakeConfigMapGetter struct {
	ConfigMap *corev1.ConfigMap
	Err       error
}

func (f FakeConfigMapGetter) Get(ctx context.Context, name string, opts metav1.GetOptions) (*corev1.ConfigMap, error) {
	if f.Err != nil {
		return nil, f.Err
	}
	return f.ConfigMap, nil
}
