package e2e

import (
	"strconv"

	"github.com/magefile/mage/sh"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	clientconfig "sigs.k8s.io/controller-runtime/pkg/client/config"
)

func getClient() (client.Client, *rest.Config, error) {
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

func objKey(ns, name string) client.ObjectKey {
	return client.ObjectKey{Namespace: ns, Name: name}
}
func deleteNS(ns string) error {
	return sh.RunV("kubectl", "delete", "namespace", ns)
}

func getPortStrings(svc *corev1.Service) []string {
	ret := []string{}
	for _, port := range svc.Spec.Ports {
		ret = append(ret, strconv.Itoa(int(port.Port)))
	}
	return ret
}
