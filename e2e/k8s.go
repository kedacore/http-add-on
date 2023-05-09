package e2e

import (
	"context"

	"github.com/codeskyblue/go-sh"
	kedav1alpha1 "github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	"github.com/pkg/errors"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	clientconfig "sigs.k8s.io/controller-runtime/pkg/client/config"
)

func getClient() (
	client.Client,
	*rest.Config,
	error,
) {
	cfg, err := clientconfig.GetConfig()
	if err != nil {
		return nil, nil, err
	}
	cl, err := client.New(cfg, client.Options{})
	if err != nil {
		return nil, nil, errors.Wrap(err, "getClient")
	}
	return cl, cfg, nil
}

func deleteNS(ns string) error {
	return sh.Command("kubectl", "delete", "namespace", ns).Run()
}

func getScaledObject(ctx context.Context, cl client.Client, ns string, name string) error {
	var scaledObject kedav1alpha1.ScaledObject
	return cl.Get(ctx, client.ObjectKey{Namespace: ns, Name: name}, &scaledObject)
}
