//go:build e2e

package interceptor_scaledobject_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	. "github.com/kedacore/http-add-on/tests/helper"
)

const (
	kedaNamespace               = "keda"
	interceptorScaledObjectName = "keda-add-ons-http-interceptor"
	scaledObject                = "scaledobject"
)

func TestCheck(t *testing.T) {
	predicate := `-o jsonpath="{.status.conditions[?(@.type=="Ready")].status}"`
	expectedResult := "True"
	result := "False"
	for range 4 {
		result = KubectlGetResult(t, scaledObject, interceptorScaledObjectName, kedaNamespace, predicate)
		if result == expectedResult {
			break
		}
		time.Sleep(15 * time.Second)
	}
	assert.Equal(t, expectedResult, result)
}
