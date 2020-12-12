package controllers

import (
	"github.com/kedacore/http-add-on/operator/api/v1alpha1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

func removeAppObjects(
	so *v1alpha1.ScaledObject,
	k8sCl *kubernetes.Clientset,
	dynamicCl dynamic.Interface,
) error {
	// TODO
	return nil
}

func addAppObjects(
	so *v1alpha1.ScaledObject,
	k8sCl kubernetes.Clientset,
	dynamicCl dynamic.Interface,
) error {
	return nil
}
