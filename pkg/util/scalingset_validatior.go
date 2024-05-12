package util

import (
	"os"

	httpv1alpha1 "github.com/kedacore/http-add-on/operator/apis/http/v1alpha1"
)

const ScalingSetNameEnv = "KEDA_HTTP_SCALING_SET_NAME"
const ScalingSetKindEnv = "KEDA_HTTP_SCALING_SET_KIND"

var (
	selfHTTPScalingSetName = ""
	selfHTTPScalingSetKind = ""
)

func init() {
	val, exist := os.LookupEnv(ScalingSetNameEnv)
	if exist {
		selfHTTPScalingSetName = val
	}
	val, exist = os.LookupEnv(ScalingSetKindEnv)
	if exist {
		selfHTTPScalingSetKind = val
	}
}

func IsManagedByThisScalingSet(httpScaledObject *httpv1alpha1.HTTPScaledObject) bool {
	scalingSet := httpScaledObject.Spec.GetHTTPSalingSetTargetRef()

	return scalingSet.Name == selfHTTPScalingSetName &&
		string(scalingSet.Kind) == selfHTTPScalingSetKind
}
