package e2e

import (
	"context"

	"github.com/kedacore/http-add-on/pkg/k8s"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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

func getScaledObject(
	ctx context.Context,
	cl client.Client,
	ns,
	name string,
) (*unstructured.Unstructured, error) {
	scaledObject, err := k8s.NewScaledObject(
		ns,
		name,
		"",
		"",
		"",
		1,
		2,
	)
	if err != nil {
		return nil, err
	}
	if err := cl.Get(ctx, k8s.ObjKey(ns, name), scaledObject); err != nil {
		return nil, err
	}
	return scaledObject, nil
}
