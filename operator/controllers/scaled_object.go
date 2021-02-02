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
		appInfo.Namespace,
		appInfo.ExternalScalerConfig.Port,
	)

	logger.Info("Creating scaled objects", "external scaler host name", externalScalerHostname)

	// create scaled object for app
	appScaledObject := k8s.NewScaledObject(
		appInfo.ScaledObjectName(),
		appInfo.Name,
		externalScalerHostname,
		0,
		1000,
	)
	logger.Info("Creating ScaledObject for app", "ScaledObject", *appScaledObject)
	scaledObjectCl := k8s.NewScaledObjectClient(K8sDynamicCl)
	if _, err := scaledObjectCl.
		Namespace(appInfo.Namespace).
		Create(appScaledObject, metav1.CreateOptions{}); err != nil {
		if errors.IsAlreadyExists(err) {
			logger.Info("User app scaled object already exists, moving on")
		} else {

			logger.Error(err, "Creating ScaledObject")
			httpso.Status.ScaledObjectStatus = v1alpha1.Error
			return err
		}
	}

	// create scaled object for interceptor. Make sure to always
	// have 1 replica available so that incoming traffic gets accepted
	interceptorScaledObject := k8s.NewScaledObject(
		appInfo.InterceptorScaledObjectName(),
		appInfo.InterceptorDeploymentName(),
		externalScalerHostname,
		1,
		1000,
	)
	logger.Info("Creating ScaledObject for interceptor", "ScaledObject", *interceptorScaledObject)
	if _, err := scaledObjectCl.
		Namespace(appInfo.Namespace).
		Create(interceptorScaledObject, metav1.CreateOptions{}); err != nil {
		if errors.IsAlreadyExists(err) {
			logger.Info("Interceptor scaled object already exists, moving on")
		} else {

			logger.Error(err, "Creating ScaledObject")
			httpso.Status.ScaledObjectStatus = v1alpha1.Error
			return err
		}
	}

	httpso.Status.ScaledObjectStatus = v1alpha1.Created
	return nil
}
