package http

import (
	"context"
	"testing"
	"time"

	kedav1alpha1 "github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kedacore/http-add-on/operator/apis/http/v1alpha1"
	"github.com/kedacore/http-add-on/operator/controllers/http/config"
)

const externalScalerHostName = "mysvc.myns.svc.cluster.local:9090"

func TestCreateOrUpdateScaledObject(t *testing.T) {
	r := require.New(t)
	testInfra := newCommonTestInfra("testns", "testapp")
	reconciller := &HTTPScaledObjectReconciler{
		Client:               testInfra.cl,
		Scheme:               testInfra.cl.Scheme(),
		ExternalScalerConfig: config.ExternalScaler{},
		BaseConfig:           config.Base{},
	}

	r.NoError(reconciller.createOrUpdateScaledObject(
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

	// check that the app ScaledObject has the correct owner
	r.Len(retSO.OwnerReferences, 1, "ScaledObject should have the owner reference")

	metadata := retSO.ObjectMeta
	spec := retSO.Spec

	r.Equal(testInfra.ns, metadata.Namespace)
	r.Equal(
		testInfra.httpso.GetName(),
		metadata.Name,
	)

	r.EqualValues(
		testInfra.httpso.ObjectMeta.Labels,
		metadata.Labels,
	)

	r.EqualValues(
		testInfra.httpso.ObjectMeta.Annotations,
		metadata.Annotations,
	)

	var minReplicaCount *int32
	var maxReplicaCount *int32
	if replicas := testInfra.httpso.Spec.Replicas; replicas != nil {
		minReplicaCount = replicas.Min
		maxReplicaCount = replicas.Max
	}

	r.EqualValues(
		minReplicaCount,
		spec.MinReplicaCount,
	)
	r.EqualValues(
		maxReplicaCount,
		spec.MaxReplicaCount,
	)

	// get hosts from spec and ensure all the hosts are there
	r.Equal(
		2,
		len(testInfra.httpso.Spec.Hosts),
	)

	// now update the min and max replicas on the httpso
	// and call createOrUpdateScaledObject again
	if spec := &testInfra.httpso.Spec; spec.Replicas == nil {
		spec.Replicas = &v1alpha1.ReplicaStruct{
			Min: new(int32),
			Max: new(int32),
		}
	}
	*testInfra.httpso.Spec.Replicas.Min++
	*testInfra.httpso.Spec.Replicas.Max++
	testInfra.httpso.ObjectMeta.Labels["test"] = "test-label"
	testInfra.httpso.ObjectMeta.Annotations["test"] = "test-annotation"
	r.NoError(reconciller.createOrUpdateScaledObject(
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

	metadata = retSO.ObjectMeta
	r.EqualValues(
		testInfra.httpso.ObjectMeta.Labels,
		metadata.Labels,
	)

	r.EqualValues(
		testInfra.httpso.ObjectMeta.Annotations,
		metadata.Annotations,
	)

	spec = retSO.Spec
	r.Equal(
		*testInfra.httpso.Spec.Replicas.Min,
		*spec.MinReplicaCount,
	)
	r.Equal(
		*testInfra.httpso.Spec.Replicas.Max,
		*spec.MaxReplicaCount,
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
		Name:      httpso.GetName(),
	}, &retSO)
	return &retSO, err
}
