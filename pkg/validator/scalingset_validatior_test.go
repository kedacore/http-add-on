package validator

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kedacore/http-add-on/operator/apis/http/v1alpha1"
)

func TestIsManagedByThisScalingSet(t *testing.T) {
	var testCases = []struct {
		name                       string
		localScalingSetName        string
		localScalingSetKind        string
		httpScaledObjectScalingSet *v1alpha1.HTTPSalingSetTargetRef
		result                     bool
	}{
		{
			name:                       "not defined in component and not set in HTTPScaledObject",
			localScalingSetName:        "",
			localScalingSetKind:        "",
			httpScaledObjectScalingSet: nil,
			result:                     true,
		},
		{
			name:                       "not defined in component and set in HTTPScaledObject",
			localScalingSetName:        "",
			localScalingSetKind:        "",
			httpScaledObjectScalingSet: &v1alpha1.HTTPSalingSetTargetRef{Name: "ss-1"},
			result:                     false,
		},
		{
			name:                       "defined in component and not set in HTTPScaledObject",
			localScalingSetName:        "ss-0",
			localScalingSetKind:        "",
			httpScaledObjectScalingSet: nil,
			result:                     false,
		},
		{
			name:                       "defined in component and set in HTTPScaledObject (different values)",
			localScalingSetName:        "ss-0",
			localScalingSetKind:        "",
			httpScaledObjectScalingSet: &v1alpha1.HTTPSalingSetTargetRef{Name: "ss-1"},
			result:                     false,
		},
		{
			name:                       "defined in component, set in HTTPScaledObject, and kind matches (unset)",
			localScalingSetName:        "ss-0",
			localScalingSetKind:        string(v1alpha1.ClusterHTTPScalingSetKind),
			httpScaledObjectScalingSet: &v1alpha1.HTTPSalingSetTargetRef{Name: "ss-0"},
			result:                     true,
		},
		{
			name:                       "defined in component, set in HTTPScaledObject, and kind matches (set)",
			localScalingSetName:        "ss-0",
			localScalingSetKind:        string(v1alpha1.HTTPScalingSetKind),
			httpScaledObjectScalingSet: &v1alpha1.HTTPSalingSetTargetRef{Name: "ss-0", Kind: v1alpha1.HTTPScalingSetKind},
			result:                     true,
		},
		{
			name:                       "defined in component, set in HTTPScaledObject, and kind doesn't match (set)",
			localScalingSetName:        "ss-0",
			localScalingSetKind:        string(v1alpha1.ClusterHTTPScalingSetKind),
			httpScaledObjectScalingSet: &v1alpha1.HTTPSalingSetTargetRef{Name: "ss-0", Kind: v1alpha1.HTTPScalingSetKind},
			result:                     false,
		},
		{
			name:                       "defined in component, set in HTTPScaledObject, and kind doesn't match (unset)",
			localScalingSetName:        "ss-0",
			localScalingSetKind:        string(v1alpha1.HTTPScalingSetKind),
			httpScaledObjectScalingSet: &v1alpha1.HTTPSalingSetTargetRef{Name: "ss-0"},
			result:                     false,
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			httpso := &v1alpha1.HTTPScaledObject{
				Spec: v1alpha1.HTTPScaledObjectSpec{
					ScalingSet: tt.httpScaledObjectScalingSet,
				},
			}
			selfHTTPScalingSetName = tt.localScalingSetName
			selfHTTPScalingSetKind = tt.localScalingSetKind
			result := IsManagedByThisScalingSet(httpso)
			assert.Equal(t, tt.result, result)
		})
	}
}
