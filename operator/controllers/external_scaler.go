package controllers

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"github.com/kedacore/http-add-on/operator/api/v1alpha1"
	"github.com/kedacore/http-add-on/operator/controllers/config"
	"github.com/kedacore/http-add-on/pkg/k8s"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// creates the external scaler and returns the cluster-DNS hostname of it.
// if something went wrong creating it, returns empty string and a non-nil error
func createExternalScaler(
	ctx context.Context,
	appInfo config.AppInfo,
	cl client.Client,
	logger logr.Logger,
	httpso *v1alpha1.HTTPScaledObject,
) (string, error) {
	scalerPort := appInfo.ExternalScalerConfig.Port
	healthCheckPort := scalerPort + 1
	scalerDeployment := k8s.NewDeployment(
		appInfo.Namespace,
		appInfo.ExternalScalerDeploymentName(),
		appInfo.ExternalScalerConfig.Image,
		[]int32{
			appInfo.ExternalScalerConfig.Port,
		},
		[]corev1.EnvVar{
			{
				Name:  "KEDA_HTTP_SCALER_PORT",
				Value: fmt.Sprintf("%d", scalerPort),
			},
			{
				Name:  "KEDA_HTTP_HEALTH_PORT",
				Value: fmt.Sprintf("%d", healthCheckPort),
			},
			{
				Name:  "KEDA_HTTP_SCALER_TARGET_ADMIN_NAMESPACE",
				Value: appInfo.Namespace,
			},
			{
				Name:  "KEDA_HTTP_SCALER_TARGET_ADMIN_SERVICE",
				Value: appInfo.InterceptorAdminServiceName(),
			},
			{
				Name:  "KEDA_HTTP_SCALER_TARGET_ADMIN_PORT",
				Value: fmt.Sprintf("%d", appInfo.InterceptorConfig.AdminPort),
			},
			{
				Name:  "KEDA_HTTP_SCALER_TARGET_METRIC",
				Value: fmt.Sprintf("%d", httpso.Spec.TargetMetric),
			},
		},
		k8s.Labels(appInfo.ExternalScalerDeploymentName()),
		appInfo.ExternalScalerConfig.PullPolicy,
	)
	if err := k8s.AddLivenessProbe(
		scalerDeployment,
		"/livez",
		int(healthCheckPort),
	); err != nil {
		logger.Error(err, "Creating liveness check")
		condition := v1alpha1.CreateCondition(v1alpha1.Error, metav1.ConditionFalse, v1alpha1.ErrorCreatingExternalScaler).SetMessage(err.Error())
		httpso.AddCondition(*condition)
		return "", err
	}

	if err := k8s.AddReadinessProbe(
		scalerDeployment,
		"/healthz",
		int(healthCheckPort),
	); err != nil {
		logger.Error(err, "Creating readiness check")
		condition := v1alpha1.CreateCondition(v1alpha1.Error, metav1.ConditionFalse, v1alpha1.ErrorCreatingExternalScaler).SetMessage(err.Error())
		httpso.AddCondition(*condition)
		return "", err
	}

	logger.Info("Creating external scaler Deployment", "Deployment", *scalerDeployment)
	if err := cl.Create(ctx, scalerDeployment); err != nil {
		if errors.IsAlreadyExists(err) {
			logger.Info("External scaler deployment already exists, moving on")
		} else {
			logger.Error(err, "Creating scaler deployment")
			condition := v1alpha1.CreateCondition(v1alpha1.Error, metav1.ConditionFalse, v1alpha1.ErrorCreatingExternalScaler).SetMessage(err.Error())
			httpso.AddCondition(*condition)
			return "", err
		}
	}

	// NOTE: Scaler port is fixed here because it's a fixed on the scaler main (@see ../scaler/main.go:17)
	servicePorts := []corev1.ServicePort{
		k8s.NewTCPServicePort(
			"externalscaler",
			appInfo.ExternalScalerConfig.Port,
			appInfo.ExternalScalerConfig.Port,
		),
	}
	scalerService := k8s.NewService(
		appInfo.Namespace,
		appInfo.ExternalScalerServiceName(),
		servicePorts,
		corev1.ServiceTypeClusterIP,
		k8s.Labels(appInfo.ExternalScalerDeploymentName()),
	)
	logger.Info("Creating external scaler Service", "Service", *scalerService)
	if err := cl.Create(ctx, scalerService); err != nil {
		if errors.IsAlreadyExists(err) {
			logger.Info("External scaler service already exists, moving on")
		} else {
			logger.Error(err, "Creating scaler service")
			condition := v1alpha1.CreateCondition(v1alpha1.Error, metav1.ConditionFalse, v1alpha1.ErrorCreatingExternalScalerService).SetMessage(err.Error())
			httpso.AddCondition(*condition)
			return "", err
		}
	}
	condition := v1alpha1.CreateCondition(v1alpha1.Created, metav1.ConditionTrue, v1alpha1.CreatedExternalScaler).SetMessage("External scaler object is created")
	httpso.AddCondition(*condition)
	externalScalerHostName := fmt.Sprintf(
		"%s.%s.svc.cluster.local:%d",
		appInfo.ExternalScalerServiceName(),
		appInfo.Namespace,
		appInfo.ExternalScalerConfig.Port,
	)
	return externalScalerHostName, nil
}

// waitForScaler uses the gRPC scaler client's IsActive call to determine
// whether the scaler is active, retrying numRetries times with retryDelay
// in between each retry.
//
// This function considers the scaler to be active when IsActive returns
// a nil error and a non-nil IsActiveResponse type. If that happens, it immediately
// returns a nil error. If that doesn't happen after all retries, returns a non-nil error.
//
// waitForScaler also establishes a gRPC client connection and may return
// a non-nil error if that fails.
func waitForScaler(
	ctx context.Context,
	cl client.Client,
	scalerDeplNS,
	scalerDeplName string,
	retries uint,
	retryDelay time.Duration,
) error {

	checkStatus := func() error {
		depl := &appsv1.Deployment{}
		if err := cl.Get(ctx, client.ObjectKey{
			Namespace: scalerDeplNS,
			Name:      scalerDeplName,
		}, depl); err != nil {
			return err
		}

		if depl.Status.ReadyReplicas > 0 {
			return nil
		}

		return fmt.Errorf(
			"No replicas ready for scaler deployment %s/%s",
			scalerDeplNS,
			scalerDeplName,
		)
	}

	// this returns an error if the context is done, so need to
	// always bail out if this gets a non-nil
	waitForRetry := func(ctx context.Context) error {
		t := time.NewTimer(retryDelay)
		defer t.Stop()
		select {
		case <-t.C:
			return nil
		case <-ctx.Done():
			return ctx.Err()

		}
	}
	for tryNum := uint(0); tryNum < retries; tryNum++ {
		statusErr := checkStatus()
		if statusErr == nil {
			return nil
		}
		if retryErr := waitForRetry(ctx); retryErr != nil {
			return retryErr
		}
	}

	return fmt.Errorf(
		"Scaler failed to start up within %d",
		retryDelay*time.Duration(retries),
	)
}
