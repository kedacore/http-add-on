//go:build e2e
// +build e2e

package interceptor_scaledobject_test

import (
	"testing"
	"time"

	. "github.com/kedacore/http-add-on/tests/helper"
	"github.com/stretchr/testify/assert"
)

const (
	kedaNamespace               = "keda"
	interceptorScaledObjectName = "keda-http-add-on-interceptor"
	scaledObject                = "scaledobject"
)

func TestCheck(t *testing.T) {
	predicate := `-o jsonpath="{.status.conditions[?(@.type=="Ready")].status}"`
	expectedResult := "True"
	result := "False"
	for i := 0; i < 4; i++ {
		result = KubectlGetResult(t, scaledObject, interceptorScaledObjectName, kedaNamespace, predicate)
		if result == expectedResult {
			break
		}
		time.Sleep(15 * time.Second)
	}
	assert.Equal(t, expectedResult, result)
}
