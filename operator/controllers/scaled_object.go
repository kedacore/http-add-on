package controllers

import (
	"fmt"

	"github.com/go-logr/logr"
	"github.com/kedacore/http-add-on/operator/api/v1alpha1"
	"github.com/kedacore/http-add-on/operator/controllers/config"
	"github.com/kedacore/http-add-on/pkg/k8s"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
)

func createScaledObject(
	appInfo config.AppInfo,
	K8sDynamicCl dynamic.Interface,
	logger logr.Logger,
	httpso *v1alpha1.HTTPScaledObject,
) error {

	externalScalerHostname := fmt.Sprintf(
		"%s.%s.svc.cluster.local:%d",
		appInfo.ExternalScalerServiceName(),
		appInfo.App.Namespace,
		appInfo.ExternalScalerConfig.Port,
	)

	logger.Info("Creating scaled object", "external_scaler", externalScalerHostname)

	coreScaledObject := k8s.NewScaledObject(
		appInfo.ScaledObjectName(),
		appInfo.App.Name,
		externalScalerHostname,
	)
	logger.Info("Creating ScaledObject", "ScaledObject", *coreScaledObject)
	// TODO: use r.Client here, not the dynamic one
	scaledObjectCl := k8s.NewScaledObjectClient(K8sDynamicCl)
	if _, err := scaledObjectCl.
		Namespace(appInfo.App.Namespace).
		Create(coreScaledObject, metav1.CreateOptions{}); err != nil {
		if errors.IsAlreadyExists(err) {
			logger.Info("User app service already exists, moving on")
		} else {

			logger.Error(err, "Creating ScaledObject")
			httpso.Status.ScaledObjectStatus = v1alpha1.Error
			return err
		}
	}
	httpso.Status.ScaledObjectStatus = v1alpha1.Created
	return nil
}
