package controllers

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	"github.com/kedacore/http-add-on/pkg/routing"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestRoutingTable(t *testing.T) {
	table := routing.NewTable()
	const (
		host     = "myhost.com"
		ns       = "testns"
		svcName  = "testsvc"
		deplName = "testdepl"
	)
	r := require.New(t)
	ctx := context.Background()
	configMap := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      "keda-http-routing-table",
		},
		Data: map[string]string{
			//An error would occur if this map is left empty.
			//It would be nil probably because of the deep copy.
			"hello": "Hello",
		},
	}
	cl := fake.NewClientBuilder().Build()
	r.NoError(cl.Create(ctx, &configMap))

	target := routing.Target{
		Service:    svcName,
		Port:       8080,
		Deployment: deplName,
	}
	r.NoError(addAndUpdateRoutingTable(
		ctx,
		logr.Discard(),
		cl,
		table,
		host,
		target,
		ns,
	))
	// TODO: ensure that the ConfigMap was updated.
	// requires
	// https://github.com/kubernetes-sigs/controller-runtime/issues/1633
	// to be implemented.

	retTarget, err := table.Lookup(host)
	r.NoError(err)
	r.Equal(target, retTarget)

	r.NoError(removeAndUpdateRoutingTable(
		ctx,
		logr.Discard(),
		cl,
		table,
		host,
		ns,
	))

	// TODO: ensure that the ConfigMap was updated.
	// requires
	// https://github.com/kubernetes-sigs/controller-runtime/issues/1633
	// to be implemnented

	_, err = table.Lookup(host)
	r.Error(err)
}
