//go:build e2e

package helpers

import (
	kedav1alpha1 "github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	httpv1beta1 "github.com/kedacore/http-add-on/operator/apis/http/v1beta1"
)

const (
	defaultMinReplicas  = int32(1)
	defaultMaxReplicas  = int32(1)
	defaultIdleReplicas = int32(0)  // enables scale-to-zero when idle
	defaultCooldown     = int32(10) // short cooldown for fast test feedback
	defaultPolling      = int32(5)  // short polling for fast test feedback
)

// SOOption configures a ScaledObject before creation.
type SOOption func(*kedav1alpha1.ScaledObject)

// CreateScaledObject creates a ScaledObject targeting the given app and IR in the cluster.
func (f *Framework) CreateScaledObject(name string, app *TestApp, ir *httpv1beta1.InterceptorRoute, opts ...SOOption) *kedav1alpha1.ScaledObject {
	f.t.Helper()
	so := &kedav1alpha1.ScaledObject{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "keda.sh/v1alpha1",
			Kind:       "ScaledObject",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: f.namespace,
		},
		Spec: kedav1alpha1.ScaledObjectSpec{
			ScaleTargetRef: &kedav1alpha1.ScaleTarget{
				Name: app.Name,
			},
			MinReplicaCount:  ptr.To(defaultMinReplicas),
			MaxReplicaCount:  ptr.To(defaultMaxReplicas),
			IdleReplicaCount: ptr.To(defaultIdleReplicas),
			CooldownPeriod:   ptr.To(defaultCooldown),
			PollingInterval:  ptr.To(defaultPolling),
			Triggers: []kedav1alpha1.ScaleTriggers{{
				Type: "external-push",
				Metadata: map[string]string{
					"interceptorRoute": ir.Name,
					"scalerAddress":    scalerAddress,
				},
			}},
		},
	}
	for _, opt := range opts {
		opt(so)
	}
	f.createResource(so)
	return so
}
