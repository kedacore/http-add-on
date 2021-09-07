package k8s

import (
	"github.com/pkg/errors"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NewClientset gets a new Kubernetes clientset, or calls log.Fatal
// if it couldn't
func NewClientset() (*kubernetes.Clientset, dynamic.Interface, error) {
	// creates the in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, nil, errors.Wrap(err, "Getting in-cluster config")
	}
	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, nil, errors.Wrap(err, "Creating k8s clientset")
	}
	dynamic, err := dynamic.NewForConfig(config)

	if err != nil {
		return nil, nil, err
	}
	return clientset, dynamic, nil
}

// ObjKey creates a new client.ObjectKey with the given
// name and namespace
func ObjKey(ns, name string) client.ObjectKey {
	return client.ObjectKey{Namespace: ns, Name: name}
}
