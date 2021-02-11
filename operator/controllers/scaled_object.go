package controllers

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/kedacore/http-add-on/operator/api/v1alpha1"
	"github.com/kedacore/http-add-on/operator/controllers/config"
	"github.com/kedacore/http-add-on/pkg/k8s"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func createScaledObject(
	ctx context.Context,
	appInfo config.AppInfo,
	cl client.Client,
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
		appInfo.Namespace,
		appInfo.ScaledObjectName(),
		appInfo.Name,
		externalScalerHostname,
	)
	logger.Info("Creating ScaledObject", "ScaledObject", *coreScaledObject)
	if err := cl.Create(ctx, coreScaledObject); err != nil {
		if errors.IsAlreadyExists(err) {
			logger.Info("User app service already exists, moving on")
		} else {
			logger.Error(err, "Creating ScaledObject")
			httpso.AddCondition(*v1alpha1.CreateCondition(v1alpha1.Error, v1.ConditionFalse, v1alpha1.ErrorCreatingScaledObject).SetMessage(err.Error()))
			return err
		}
	}
	httpso.AddCondition(*v1alpha1.CreateCondition(v1alpha1.Created, v1.ConditionTrue, v1alpha1.ScaledObjectCreated).SetMessage("Scaled object created"))
	return nil
}
