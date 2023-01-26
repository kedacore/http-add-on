package e2e

import (
	"context"

	"github.com/codeskyblue/go-sh"
	"github.com/pkg/errors"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	clientconfig "sigs.k8s.io/controller-runtime/pkg/client/config"

	"github.com/kedacore/http-add-on/pkg/k8s"
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

func getScaledObject(
	ctx context.Context,
	cl client.Client,
	ns,
	name string,
) error {
	scaledObject, err := k8s.NewScaledObject(
		ns,
		name,
		"",
		"",
		[]string{""},
		1,
		2,
		30,
	)
	if err != nil {
		return err
	}
	if err := cl.Get(ctx, k8s.ObjKey(ns, name), scaledObject); err != nil {
		return err
	}
	return nil
}
