package routing

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	"github.com/kedacore/http-add-on/pkg/queue"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func newTableFromMap(m map[string]Target) *Table {
	table := NewTable()
	for host, target := range m {
		table.AddTarget(host, target)
	}
	return table
}

func TestRPCIntegration(t *testing.T) {
	const ns = "testns"
	ctx := context.Background()
	lggr := logr.Discard()
	r := require.New(t)

	// fetch an empty table
	retTable := NewTable()
	k8sCl, err := fakeConfigMapClientForTable(
		NewTable(),
		ns,
		ConfigMapRoutingTableName,
	)
	r.NoError(err)
	r.NoError(GetTable(
		ctx,
		lggr,
		k8sCl.CoreV1().ConfigMaps("testns"),
		retTable,
		queue.NewFakeCounter(),
	))
	r.Equal(0, len(retTable.m))

	// fetch a table with lots of targets in it
	targetMap := map[string]Target{
		"host1": {
			Service:    "svc1",
			Port:       1234,
			Deployment: "depl1",
		},
		"host2": {
			Service:    "svc2",
			Port:       2345,
			Deployment: "depl2",
		},
	}

	retTable = NewTable()
	k8sCl, err = fakeConfigMapClientForTable(
		newTableFromMap(targetMap),
		ns,
		ConfigMapRoutingTableName,
	)
	r.NoError(err)
	r.NoError(GetTable(
		ctx,
		lggr,
		k8sCl.CoreV1().ConfigMaps("testns"),
		retTable,
		queue.NewFakeCounter(),
	))
	r.Equal(len(targetMap), len(retTable.m))
	r.Equal(targetMap, retTable.m)
}

func fakeConfigMapClientForTable(t *Table, ns, name string) (*fake.Clientset, error) {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Data: map[string]string{},
	}
	if err := SaveTableToConfigMap(t, cm); err != nil {
		return nil, err
	}

	return fake.NewSimpleClientset(cm), nil
}
