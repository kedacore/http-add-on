package controllers

import (
	"fmt"
	"testing"
	"time"

	"github.com/kedacore/http-add-on/operator/api/v1alpha1"
	"github.com/kedacore/http-add-on/operator/controllers/config"
	"github.com/kedacore/http-add-on/pkg/k8s"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestCreateOrUpdateScaledObject(t *testing.T) {
	r := require.New(t)
	const externalScalerHostName = "mysvc.myns.svc.cluster.local:9090"

	testInfra := newCommonTestInfra("testns", "testapp")
	err := createOrUpdateScaledObject(
		testInfra.ctx,
		testInfra.cl,
		testInfra.logger,
		externalScalerHostName,
		&testInfra.httpso,
	)
	r.NoError(err)

	// make sure that httpso has the AppScaledObjectCreated
	// condition on it
	r.Equal(1, len(testInfra.httpso.Status.Conditions))

	cond1 := testInfra.httpso.Status.Conditions[0]
	cond1ts, err := time.Parse(time.RFC3339, cond1.Timestamp)
	r.NoError(err)
	r.GreaterOrEqual(time.Since(cond1ts), time.Duration(0))
	r.Equal(v1alpha1.Created, cond1.Type)
	r.Equal(metav1.ConditionTrue, cond1.Status)
	r.Equal(v1alpha1.AppScaledObjectCreated, cond1.Reason)

	// check that the app ScaledObject was created
	retSO := k8s.NewEmptyScaledObject()
	err = testInfra.cl.Get(testInfra.ctx, client.ObjectKey{
		Namespace: testInfra.ns,
		Name:      config.AppScaledObjectName(&testInfra.httpso),
	}, retSO)
	r.NoError(err)

	metadata, err := getKeyAsMap(retSO.Object, "metadata")
	r.NoError(err)
	r.Equal(testInfra.ns, metadata["namespace"])
	r.Equal(
		config.AppScaledObjectName(&testInfra.httpso),
		metadata["name"],
	)

	spec, err := getKeyAsMap(retSO.Object, "spec")
	r.NoError(err)
	// HTTPScaledObject min/max replicas are int32s,
	// but the ScaledObject's spec is decoded into
	// an *unsructured.Unstructured (basically a map[string]interface{})
	// which is an int64. we need to convert the
	// HTTPScaledObject's values into int64s before we compare
	r.Equal(
		int64(testInfra.httpso.Spec.Replicas.Min),
		spec["minReplicaCount"],
	)
	r.Equal(
		int64(testInfra.httpso.Spec.Replicas.Max),
		spec["maxReplicaCount"],
	)

}

func getKeyAsMap(m map[string]interface{}, key string) (map[string]interface{}, error) {
	iface, ok := m[key]
	if !ok {
		return nil, fmt.Errorf("key %s not found in map", key)
	}
	val, ok := iface.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("key %s was not a map[string]interface{}", key)
	}
	return val, nil

}
