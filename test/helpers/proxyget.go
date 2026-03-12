//go:build e2e

package helpers

import (
	"fmt"

	"k8s.io/client-go/kubernetes"
)

// ServiceProxyGet performs an HTTP GET to a cluster service via the API server
// proxy. The port can be a number or a named port (e.g. "query").
func (f *Framework) ServiceProxyGet(namespace, service, port, path string, params map[string]string) ([]byte, error) {
	clientset, err := kubernetes.NewForConfig(f.client.RESTConfig())
	if err != nil {
		return nil, fmt.Errorf("failed to create clientset: %w", err)
	}

	body, err := clientset.CoreV1().Services(namespace).
		ProxyGet("http", service, port, path, params).
		DoRaw(f.ctx)
	if err != nil {
		return nil, fmt.Errorf("ProxyGet %s/%s:%s%s: %w", namespace, service, port, path, err)
	}

	return body, nil
}
