package http

import (
	"context"
	"fmt"
	"strconv"

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
	interceptor string = "interceptor"
	proxy       string = "proxy"
	admin       string = "admin"
)

var (
	interceptorProxyName = fmt.Sprintf("%s-%s", interceptor, proxy)
	interceptorAdminName = fmt.Sprintf("%s-%s", interceptor, admin)
)

func getInterceptorServiceNames(scalingSetName string) (proxy, admin string) {
	proxy = fmt.Sprintf("%s-%s", scalingSetName, interceptorProxyName)
	admin = fmt.Sprintf("%s-%s", scalingSetName, interceptorAdminName)
	return
}

func createOrUpdateInterceptorResources(
	ctx context.Context,
	logger logr.Logger,
	cl client.Client,
	httpss metav1.Object,
	scheme *runtime.Scheme,
) error {
	proxyServiceName, adminServiceName := getInterceptorServiceNames(httpss.GetName())
	interceptorName := fmt.Sprintf("%s-%s", httpss.GetName(), interceptor)
	httpSpec := util.GetHTTPScalingSetSpecFromObject(httpss)
	httpKind := util.GetHTTPScalingSetKindFromObject(httpss)

	logger = logger.WithValues(
		"reconciler.appObjects",
		"addObjects",
		"HTTPScalingSet.name",
		httpss.GetName(),
	)
	selector := map[string]string{
		"http.keda.sh/scaling-set":           httpss.GetName(),
		"http.keda.sh/scaling-set-component": interceptor,
		"http.keda.sh/scaling-set-kind":      string(httpKind),
	}

	proxyService := k8s.NewService(proxyServiceName, httpss.GetNamespace(), proxy, httpSpec.Interceptor.GetProxyPort(), selector)
	// Set HTTPScaledObject instance as the owner and controller
	if err := controllerutil.SetControllerReference(httpss, proxyService, scheme); err != nil {
		return err
	}
	err := createOrUpdateService(ctx, logger, cl, proxyService)
	if err != nil {
		return err
	}
	adminService := k8s.NewService(adminServiceName, httpss.GetNamespace(), admin, httpSpec.Interceptor.GetAdminPort(), selector)
	// Set HTTPScaledObject instance as the owner and controller
	if err := controllerutil.SetControllerReference(httpss, adminService, scheme); err != nil {
		return err
	}
	err = createOrUpdateService(ctx, logger, cl, adminService)
	if err != nil {
		return err
	}
	ports := []corev1.ContainerPort{
		{
			Name:          proxy,
			ContainerPort: httpSpec.Interceptor.GetProxyPort(),
			Protocol:      "TCP",
		},
		{
			Name:          admin,
			ContainerPort: httpSpec.Interceptor.GetAdminPort(),
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
			Name:  "KEDA_HTTP_CURRENT_NAMESPACE",
			Value: httpss.GetNamespace(),
		},
		{
			Name:  "KEDA_HTTP_PROXY_PORT",
			Value: fmt.Sprintf("%d", httpSpec.Interceptor.GetProxyPort()),
		},
		{
			Name:  "KEDA_HTTP_ADMIN_PORT",
			Value: fmt.Sprintf("%d", httpSpec.Interceptor.GetAdminPort()),
		},
		{
			Name:  "KEDA_HTTP_CONNECT_TIMEOUT",
			Value: httpSpec.Interceptor.GetConnectTimeout(),
		},
		{
			Name:  "KEDA_HTTP_KEEP_ALIVE",
			Value: httpSpec.Interceptor.GetKeepAlive(),
		},
		{
			Name:  "KEDA_RESPONSE_HEADER_TIMEOUT",
			Value: httpSpec.Interceptor.GetHeaderTimeout(),
		},
		{
			Name:  "KEDA_CONDITION_WAIT_TIMEOUT",
			Value: httpSpec.Interceptor.GetWaitTimeout(),
		},
		{
			Name:  "KEDA_HTTP_ENDPOINTS_CACHE_POLLING_INTERVAL_MS",
			Value: strconv.Itoa(httpSpec.Interceptor.GetPollingInterval()),
		},
		{
			Name:  "KEDA_HTTP_FORCE_HTTP2",
			Value: strconv.FormatBool(httpSpec.Interceptor.GetForceHTTP2()),
		},
		{
			Name:  "KEDA_HTTP_MAX_IDLE_CONNS",
			Value: fmt.Sprintf("%d", httpSpec.Interceptor.GetMaxIdleConns()),
		},
		{
			Name:  "KEDA_HTTP_IDLE_CONN_TIMEOUT",
			Value: httpSpec.Interceptor.GetIdleConnTimeout(),
		},
		{
			Name:  "KEDA_HTTP_TLS_HANDSHAKE_TIMEOUT",
			Value: httpSpec.Interceptor.GetTLSHandshakeTimeout(),
		},
		{
			Name:  "KEDA_HTTP_EXPECT_CONTINUE_TIMEOUT",
			Value: httpSpec.Interceptor.GetExpectContinueTimeout(),
		},
	}
	interceptorDeployment := k8s.NewDeployment(
		interceptorName,
		httpss.GetNamespace(),
		httpSpec.Interceptor.ServiceAccountName,
		httpSpec.Interceptor.GetImage(),
		ports,
		envs,
		httpSpec.Interceptor.Replicas,
		selector,
		httpss.GetLabels(),
		httpss.GetAnnotations(),
		httpSpec.Interceptor.Resources,
	)
	// Set HTTPScaledObject instance as the owner and controller
	if err := controllerutil.SetControllerReference(httpss, interceptorDeployment, scheme); err != nil {
		return err
	}
	err = createOrUpdateDeployment(ctx, logger, cl, interceptorDeployment)
	if err != nil {
		return err
	}

	return nil
}
