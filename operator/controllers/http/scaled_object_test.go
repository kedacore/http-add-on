package http

import (
	"context"
	"testing"

	kedav1alpha1 "github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	"github.com/stretchr/testify/require"
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

	r.Equal(
		testInfra.httpso.Labels,
		metadata.Labels,
	)

	r.Equal(
		testInfra.httpso.Annotations,
		metadata.Annotations,
	)

	var minReplicaCount *int32
	var maxReplicaCount *int32
	if replicas := testInfra.httpso.Spec.Replicas; replicas != nil {
		minReplicaCount = replicas.Min
		maxReplicaCount = replicas.Max
	}

	r.Equal(
		minReplicaCount,
		spec.MinReplicaCount,
	)
	r.Equal(
		maxReplicaCount,
		spec.MaxReplicaCount,
	)

	// get hosts from spec and ensure all the hosts are there
	r.Len(
		testInfra.httpso.Spec.Hosts, 2,
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
	testInfra.httpso.Labels["test"] = "test-label"
	testInfra.httpso.Annotations["test"] = "test-annotation"
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

	r.Equal(
		testInfra.httpso.Labels,
		retSO.Labels,
	)

	r.Equal(
		testInfra.httpso.Annotations,
		retSO.Annotations,
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
