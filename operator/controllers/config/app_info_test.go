package config

import (
	"fmt"
	"testing"

	"github.com/kedacore/http-add-on/operator/api/v1alpha1"
	"github.com/stretchr/testify/require"
)

func TestAppScaledObjectName(t *testing.T) {
	r := require.New(t)
	obj := &v1alpha1.HTTPScaledObject{
		Spec: v1alpha1.HTTPScaledObjectSpec{
			ScaleTargetRef: &v1alpha1.ScaleTargetRef{
				Deployment: "TestAppScaledObjectNameDeployment",
			},
		},
	}
	name := AppScaledObjectName(obj)
	r.Equal(
		fmt.Sprintf(
			"%s-app",
			obj.Spec.ScaleTargetRef.Deployment,
		),
		name,
	)
}
