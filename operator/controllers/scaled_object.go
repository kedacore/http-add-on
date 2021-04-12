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

// create ScaledObjects for the app and interceptor
func createScaledObjects(
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

	logger.Info("Creating scaled objects", "external scaler host name", externalScalerHostname)

	appScaledObject, appErr := k8s.NewScaledObject(
		appInfo.Namespace,
		config.AppScaledObjectName(httpso),
		appInfo.Name,
		externalScalerHostname,
		httpso.Spec.Replicas.Min,
		httpso.Spec.Replicas.Max,
	)
	if appErr != nil {
		return appErr
	}

	interceptorScaledObject, interceptorErr := k8s.NewScaledObject(
		appInfo.Namespace,
		config.InterceptorScaledObjectName(httpso),
		appInfo.InterceptorDeploymentName(),
		externalScalerHostname,
		httpso.Spec.Replicas.Min,
		httpso.Spec.Replicas.Max,
	)
	if interceptorErr != nil {
		return interceptorErr
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

	// Interceptor ScaledObject
	logger.Info("Creating Interceptor ScaledObject", "ScaledObject", *interceptorScaledObject)
	if err := cl.Create(ctx, interceptorScaledObject); err != nil {
		if errors.IsAlreadyExists(err) {
			logger.Info("Interceptor ScaledObject already exists, moving on")
		} else {
			logger.Error(err, "Creating Interceptor ScaledObject")
			httpso.AddCondition(*v1alpha1.CreateCondition(
				v1alpha1.Error,
				v1.ConditionFalse,
				v1alpha1.ErrorCreatingInterceptorScaledObject,
			).SetMessage(err.Error()))
			return err
		}
	}

	httpso.AddCondition(*v1alpha1.CreateCondition(
		v1alpha1.Created,
		v1.ConditionTrue,
		v1alpha1.InterceptorScaledObjectCreated,
	).SetMessage("Interceptor Scaled object created"))

	return nil
}
