package controllers

import (
	"context"
	"testing"
	"time"

	kedav1alpha1 "github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kedacore/http-add-on/operator/api/v1alpha1"
	"github.com/kedacore/http-add-on/operator/controllers/config"
)

func TestCreateOrUpdateScaledObject(t *testing.T) {
	r := require.New(t)
	const externalScalerHostName = "mysvc.myns.svc.cluster.local:9090"

	testInfra := newCommonTestInfra("testns", "testapp")
	r.NoError(createOrUpdateScaledObject(
		testInfra.ctx,
		testInfra.cl,
		testInfra.logger,
		externalScalerHostName,
		&testInfra.httpso,
	))

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
	retSO, err := getSO(
		testInfra.ctx,
		testInfra.cl,
		testInfra.httpso,
	)
	r.NoError(err)

	metadata := retSO.ObjectMeta
	spec := retSO.Spec

	r.Equal(testInfra.ns, metadata.Namespace)
	r.Equal(
		config.AppScaledObjectName(&testInfra.httpso),
		metadata.Name,
	)

	// HTTPScaledObject min/max/idle replicas are int32s,
	// but the ScaledObject's spec is decoded into
	// an *unsructured.Unstructured (basically a map[string]interface{})
	// which is an int64. we need to convert the
	// HTTPScaledObject's values into int64s before we compare
	r.Equal(
		int64(testInfra.httpso.Spec.Replicas.Min),
		spec["minReplicaCount"],
	)
	r.EqualValues(
		maxReplicaCount,
		spec.MaxReplicaCount,
	)
	r.Equal(
		int64(testInfra.httpso.Spec.Replicas.Idle),
		spec["idleReplicaCount"],
	)

	// now update the min and max replicas on the httpso
	// and call createOrUpdateScaledObject again
	testInfra.httpso.Spec.Replicas.Min++
	testInfra.httpso.Spec.Replicas.Max++
	testInfra.httpso.Spec.Replicas.Idle++
	r.NoError(createOrUpdateScaledObject(
		testInfra.ctx,
		testInfra.cl,
		testInfra.logger,
		externalScalerHostName,
		&testInfra.httpso,
	))

	// get the scaledobject again and ensure it has
	// the new min/max replicas
	retSO, err = getSO(
		testInfra.ctx,
		testInfra.cl,
		testInfra.httpso,
	)
	r.NoError(err)
	spec = retSO.Spec
	r.Equal(
		*testInfra.httpso.Spec.Replicas.Min,
		*spec.MinReplicaCount,
	)
	r.Equal(
		*testInfra.httpso.Spec.Replicas.Max,
		*spec.MaxReplicaCount,
	)
	r.Equal(
		int64(testInfra.httpso.Spec.Replicas.Idle),
		spec["idleReplicaCount"],
	)
}

func getSO(
	ctx context.Context,
	cl client.Client,
	httpso v1alpha1.HTTPScaledObject,
) (*kedav1alpha1.ScaledObject, error) {
	var retSO kedav1alpha1.ScaledObject
	err := cl.Get(ctx, client.ObjectKey{
		Namespace: httpso.GetNamespace(),
		Name:      config.AppScaledObjectName(&httpso),
	}, &retSO)
	return &retSO, err
}
