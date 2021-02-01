package controllers

import (
	"fmt"

	"github.com/go-logr/logr"
	"github.com/kedacore/http-add-on/operator/api/v1alpha1"
	"github.com/kedacore/http-add-on/operator/controllers/config"
	"github.com/kedacore/http-add-on/pkg/k8s"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
		appInfo.Namespace,
		appInfo.ExternalScalerConfig.Port,
	)

	logger.Info("Creating scaled object", "external_scaler", externalScalerHostname)

	coreScaledObject := k8s.NewScaledObject(
		appInfo.ScaledObjectName(),
		appInfo.Name,
		externalScalerHostname,
	)
	logger.Info("Creating ScaledObject", "ScaledObject", *coreScaledObject)
	// TODO: use r.Client here, not the dynamic one
	scaledObjectCl := k8s.NewScaledObjectClient(K8sDynamicCl)
	if _, err := scaledObjectCl.
		Namespace(appInfo.Namespace).
		Create(coreScaledObject, v1.CreateOptions{}); err != nil {
		if errors.IsAlreadyExists(err) {
			logger.Info("User app service already exists, moving on")
		} else {

			logger.Error(err, "Creating ScaledObject")
			httpso.AddCondition(*v1alpha1.CreateCondition(v1alpha1.Error,v1.ConditionFalse,v1alpha1.ErrorCreatingScaledObject).SetMessage(err.Error()))
			return err
		}
	}
	httpso.AddCondition(*v1alpha1.CreateCondition(v1alpha1.Created,v1.ConditionTrue,v1alpha1.ScaledObjectCreated).SetMessage("Scaled object created"))
	return nil
}
