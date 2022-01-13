package config

import (
	"fmt"

	"github.com/kedacore/http-add-on/operator/api/v1alpha1"
)

// AppScaledObjectName returns the name of the ScaledObject
// that should be created alongside the given HTTPScaledObject.
func AppScaledObjectName(httpso *v1alpha1.HTTPScaledObject) string {
	return fmt.Sprintf("%s-app", httpso.Spec.ScaleTargetRef.Deployment)
}
