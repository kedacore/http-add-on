package http

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/kedacore/http-add-on/pkg/k8s"
	"github.com/kedacore/http-add-on/pkg/util"
)

const (
	externalScaler string = "external-scaler"
	grpc           string = "grpc"
)

func createOrUpdateExternalScalerResources(
	ctx context.Context,
	logger logr.Logger,
	cl client.Client,
	httpss metav1.Object,
	scheme *runtime.Scheme,
) error {
	scalerName := fmt.Sprintf("%s-%s", httpss.GetName(), externalScaler)
	_, adminServiceName := getInterceptorServiceNames(httpss.GetName())
	httpSpec := util.GetHTTPScalingSetSpecFromObject(httpss)
	httpKind := util.GetHTTPScalingSetKindFromObject(httpss)

	logger = logger.WithValues(
		"reconciler.appObjects",
		"addObjects",
		"HTTPScalingSet.name",
		httpss.GetName(),
	)
	// defer SaveStatus(context.Background(), logger, cl, httpss)
	selector := map[string]string{
		"http.keda.sh/scaling-set":           httpss.GetName(),
		"http.keda.sh/scaling-set-component": externalScaler,
		"http.keda.sh/scaling-set-kind":      string(httpKind),
	}

	scalerService := k8s.NewService(scalerName, httpss.GetNamespace(), grpc, httpSpec.Scaler.GetPort(), selector)
	// Set HTTPScaledObject instance as the owner and controller
	if err := controllerutil.SetControllerReference(httpss, scalerService, scheme); err != nil {
		return err
	}
	err := createOrUpdateService(ctx, logger, cl, scalerService)
	if err != nil {
		return err
	}
	ports := []corev1.ContainerPort{
		{
			Name:          grpc,
			ContainerPort: httpSpec.Scaler.GetPort(),
			Protocol:      "TCP",
		},
	}
	envs := []corev1.EnvVar{
		{
			Name:  util.ScalingSetNameEnv,
			Value: httpss.GetName(),
		},
		{
			Name:  util.ScalingSetKindEnv,
			Value: string(httpKind),
		},
		{
			Name:  "KEDA_HTTP_SCALER_TARGET_ADMIN_NAMESPACE",
			Value: httpss.GetNamespace(),
		},
		{
			Name:  "KEDA_HTTP_SCALER_PORT",
			Value: fmt.Sprintf("%d", httpSpec.Scaler.GetPort()),
		},
		{
			Name:  "KEDA_HTTP_SCALER_TARGET_ADMIN_SERVICE",
			Value: adminServiceName,
		},
		{
			Name:  "KEDA_HTTP_SCALER_TARGET_ADMIN_PORT",
			Value: fmt.Sprintf("%d", httpSpec.Interceptor.GetAdminPort()),
		},
	}
	scalerDeployment := k8s.NewDeployment(
		scalerName,
		httpss.GetNamespace(),
		httpSpec.Scaler.ServiceAccountName,
		httpSpec.Scaler.GetImage(),
		ports,
		envs,
		httpSpec.Scaler.Replicas,
		selector,
		httpss.GetLabels(),
		httpss.GetAnnotations(),
		httpSpec.Scaler.Resources,
	)
	// Set HTTPScaledObject instance as the owner and controller
	if err := controllerutil.SetControllerReference(httpss, scalerDeployment, scheme); err != nil {
		return err
	}
	err = createOrUpdateDeployment(ctx, logger, cl, scalerDeployment)
	if err != nil {
		return err
	}

	return nil
}
