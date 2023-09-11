//go:build e2e
// +build e2e

package interceptor_scaledobject_test

import (
	"testing"

	. "github.com/kedacore/http-add-on/tests/helper"
)

const (
	kedaNamespace               = "keda"
	interceptorScaledObjectName = "keda-http-add-on-interceptor"
	scaledObject                = "scaledobject"
)

func TestCheck(t *testing.T) {
	predicate := `-o jsonpath="{.status.conditions[?(@.type=="Ready")].status}"`
	expectedResult := "True"
	CheckKubectlGetResult(t, scaledObject, interceptorScaledObjectName, kedaNamespace, predicate, expectedResult)
}
