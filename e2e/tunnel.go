package e2e

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

// create a new port forwarder for the given service on all of its
// ports and return it.
//
// thanks to the following article for inspiration:
// https://gianarb.it/blog/programmatically-kube-port-forward-in-go
func tunnelSvc(
	t *testing.T,
	ctx context.Context,
	cfg *rest.Config,
	svc *corev1.Service,
) (*portforward.PortForwarder, error) {
	t.Helper()
	path := fmt.Sprintf(
		"/api/v1/namespaces/%s/services/%s/portforward",
		svc.Namespace,
		svc.Name,
	)
	hostIP := strings.TrimLeft(cfg.Host, "htps:/")
	t.Logf("tunneling to service %s via host %s", path, hostIP)

	transport, upgrader, err := spdy.RoundTripperFor(cfg)
	if err != nil {
		return nil, errors.Wrap(err, "tunnelSvc creating spdy RoundTripper")
	}
	dialer := spdy.NewDialer(
		upgrader,
		&http.Client{Transport: transport},
		http.MethodPost,
		&url.URL{Scheme: "https", Path: path, Host: hostIP},
	)

	readyCh := make(chan struct{})
	ports := getPortStrings(svc)
	return portforward.New(dialer, ports, ctx.Done(), readyCh, os.Stdout, os.Stdout)
}
