//go:build e2e

package helpers

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

const (
	proxyListenPort      = 8080
	proxyNamespacePrefix = "e2e-proxy"
	// renovate: datasource=docker
	proxyPodImage  = "alpine/socat:1.8.0.3"
	proxyPodPrefix = "e2e-proxy"
)

// registerInterceptorProxy registers setup/teardown funcs on testenv that manage
// the proxy pod and its port-forward to the interceptor.
// We use socat instead of a direct port-forward to route through the interceptor
// service, which allows load-balancing across multiple interceptor pods.
func registerInterceptorProxy(testenv env.Environment, interceptorPort int) {
	var stopChan chan struct{}
	proxyNamespace := randomName(proxyNamespacePrefix)

	setup := func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
		client := cfg.Client()

		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name:   proxyNamespace,
				Labels: e2eLabels,
			},
		}
		if err := client.Resources().Create(ctx, ns); err != nil && !errors.IsAlreadyExists(err) {
			return ctx, fmt.Errorf("failed to create proxy namespace: %w", err)
		}

		target := fmt.Sprintf("TCP:%s.%s.svc.cluster.local:%d", interceptorProxyService, AddonNamespace, interceptorPort)

		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: fmt.Sprintf("%s-to-%d-", proxyPodPrefix, proxyListenPort),
				Namespace:    proxyNamespace,
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{
					Name:    "socat",
					Image:   proxyPodImage,
					Command: []string{"socat"},
					// fork: handle each connection in a child process so multiple requests can be served concurrently.
					Args: []string{fmt.Sprintf("TCP-LISTEN:%d,fork,reuseaddr", proxyListenPort), target},
					Ports: []corev1.ContainerPort{{
						ContainerPort: proxyListenPort,
					}},
					ReadinessProbe: &corev1.Probe{
						ProbeHandler: corev1.ProbeHandler{
							TCPSocket: &corev1.TCPSocketAction{
								Port: intstr.FromInt32(proxyListenPort),
							},
						},
						PeriodSeconds: 1,
					},
				}},
			},
		}

		if err := client.Resources().Create(ctx, pod); err != nil {
			return ctx, fmt.Errorf("failed to create proxy pod: %w", err)
		}

		if err := wait.For(
			conditions.New(client.Resources()).PodReady(pod),
			wait.WithTimeout(defaultWaitTimeout),
		); err != nil {
			return ctx, fmt.Errorf("proxy pod not ready: %w", err)
		}

		addr, stop, err := startPortForward(cfg.Client().RESTConfig(), proxyNamespace, pod.Name, proxyListenPort)
		if err != nil {
			return ctx, fmt.Errorf("failed to port-forward to proxy pod: %w", err)
		}
		stopChan = stop
		log.Printf("port-forward: %s -> %s/%s:%d", addr, proxyNamespace, pod.Name, proxyListenPort)

		ctx = contextWithProxyAddr(ctx, addr)
		return ctx, nil
	}

	teardown := func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
		if stopChan != nil {
			close(stopChan)
		}

		client := cfg.Client()
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: proxyNamespace},
		}
		if err := client.Resources().Delete(ctx, ns); err != nil && !errors.IsNotFound(err) {
			return ctx, fmt.Errorf("failed to delete proxy namespace: %w", err)
		}

		return ctx, nil
	}

	testenv.Setup(setup)
	testenv.Finish(teardown)
}

// startPortForward sets up a port-forward to a pod and returns the
// localhost:port address and a stop channel. Close the channel to terminate
// the port-forward.
func startPortForward(restConfig *rest.Config, ns, podName string, remotePort int) (string, chan struct{}, error) {
	transport, upgrader, err := spdy.RoundTripperFor(restConfig)
	if err != nil {
		return "", nil, fmt.Errorf("failed to create SPDY round-tripper: %w", err)
	}

	parsedURL, err := url.Parse(restConfig.Host)
	if err != nil {
		return "", nil, fmt.Errorf("failed to parse REST config host: %w", err)
	}
	serverURL := url.URL{
		Scheme: parsedURL.Scheme,
		Host:   parsedURL.Host,
		Path:   fmt.Sprintf("/api/v1/namespaces/%s/pods/%s/portforward", ns, podName),
	}
	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, http.MethodPost, &serverURL)

	stopChan := make(chan struct{}, 1)
	readyChan := make(chan struct{}, 1)
	fw, err := portforward.New(dialer, []string{fmt.Sprintf("0:%d", remotePort)}, stopChan, readyChan, io.Discard, io.Discard)
	if err != nil {
		return "", nil, fmt.Errorf("failed to create port-forwarder: %w", err)
	}

	errChan := make(chan error, 1)
	go func() { errChan <- fw.ForwardPorts() }()

	select {
	case <-readyChan:
	case err := <-errChan:
		return "", nil, fmt.Errorf("port-forward failed: %w", err)
	}

	ports, err := fw.GetPorts()
	if err != nil {
		close(stopChan)
		return "", nil, fmt.Errorf("failed to get forwarded ports: %w", err)
	}

	addr := fmt.Sprintf("localhost:%d", ports[0].Local)
	return addr, stopChan, nil
}
