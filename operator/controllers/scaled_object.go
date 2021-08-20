package controllers

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/kedacore/http-add-on/operator/api/v1alpha1"
	"github.com/kedacore/http-add-on/operator/controllers/config"
	"github.com/kedacore/http-add-on/pkg/k8s"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// create ScaledObjects for the app and interceptor
func createScaledObjects(
	ctx context.Context,
	appInfo config.AppInfo,
	cl client.Client,
	logger logr.Logger,
	externalScalerHostName string,
	httpso *v1alpha1.HTTPScaledObject,
) error {

	logger.Info("Creating scaled objects", "external scaler host name", externalScalerHostName)

	appScaledObject, appErr := k8s.NewScaledObject(
		appInfo.Namespace,
		config.AppScaledObjectName(httpso),
		appInfo.Name,
		externalScalerHostName,
		httpso.Spec.Host,
		httpso.Spec.Replicas.Min,
		httpso.Spec.Replicas.Max,
	)
	if appErr != nil {
		return appErr
	}

	logger.Info("Creating App ScaledObject", "ScaledObject", *appScaledObject)
	if err := cl.Create(ctx, appScaledObject); err != nil {
		if errors.IsAlreadyExists(err) {
			logger.Info("User app scaled object already exists, moving on")
		} else {
			logger.Error(err, "Creating ScaledObject")
			httpso.AddCondition(*v1alpha1.CreateCondition(
				v1alpha1.Error,
				v1.ConditionFalse,
				v1alpha1.ErrorCreatingAppScaledObject,
			).SetMessage(err.Error()))
			return err
		}
	}

	httpso.AddCondition(*v1alpha1.CreateCondition(
		v1alpha1.Created,
		v1.ConditionTrue,
		v1alpha1.AppScaledObjectCreated,
	).SetMessage("App ScaledObject created"))

	return nil
}
