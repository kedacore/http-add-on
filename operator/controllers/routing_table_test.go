package controllers

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	"github.com/kedacore/http-add-on/pkg/routing"
	"github.com/stretchr/testify/require"
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
	cl := fake.NewClientBuilder().Build()
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
