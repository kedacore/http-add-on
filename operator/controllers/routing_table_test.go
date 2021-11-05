package controllers

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	"github.com/kedacore/http-add-on/pkg/k8s"
	"github.com/kedacore/http-add-on/pkg/routing"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
	cl := k8s.NewFakeRuntimeClient()
	cm := &corev1.ConfigMap{
		Data: map[string]string{},
	}
	// save the empty routing table to the config map,
	// so that it has valid structure
	r.NoError(routing.SaveTableToConfigMap(table, cm))
	cl.GetFunc = func() client.Object {
		return cm
	}
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
	// ensure that the ConfigMap was read and created. no updates
	// should occur
	r.Equal(1, len(cl.FakeRuntimeClientReader.GetCalls))
	r.Equal(1, len(cl.FakeRuntimeClientWriter.Patches))
	r.Equal(0, len(cl.FakeRuntimeClientWriter.Updates))
	r.Equal(0, len(cl.FakeRuntimeClientWriter.Creates))

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

	// ensure that the ConfigMap was read and updated. no additional
	// creates should occur.
	r.Equal(2, len(cl.FakeRuntimeClientReader.GetCalls))
	r.Equal(2, len(cl.FakeRuntimeClientWriter.Patches))
	r.Equal(0, len(cl.FakeRuntimeClientWriter.Updates))
	r.Equal(0, len(cl.FakeRuntimeClientWriter.Creates))

	_, err = table.Lookup(host)
	r.Error(err)
}
