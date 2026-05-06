//go:build e2e

package helpers

import (
	"context"
	"crypto/x509"
	"testing"

	"sigs.k8s.io/e2e-framework/klient"
	"sigs.k8s.io/e2e-framework/klient/k8s"
)

// Framework provides per-test helpers for e2e tests. Create one at the top of
// each Setup/Assess step via NewFramework.
type Framework struct {
	ctx       context.Context
	t         *testing.T
	client    klient.Client
	namespace string
	proxyAddr string
	CAPool    *x509.CertPool
	caIssuer  string
}

// NewFramework assembles a Framework from the context values set by the test
// environment lifecycle hooks. Call this at the top of every Setup/Assess step.
func NewFramework(ctx context.Context, t *testing.T) *Framework {
	t.Helper()
	return &Framework{
		ctx:       ctx,
		t:         t,
		client:    clientFromContext(ctx),
		namespace: namespaceFromContext(ctx),
		proxyAddr: proxyAddrFromContext(ctx),
		caIssuer:  caIssuerFromContext(ctx),
		CAPool:    caPoolFromContext(ctx),
	}
}

// Namespace returns the per-test namespace name.
func (f *Framework) Namespace() string {
	return f.namespace
}

// Hostname returns a unique hostname scoped to this test's namespace to avoid hostname conflicts.
//
//	f.Hostname()           => "e2e-hostrouting-ab12x.test"
//	f.Hostname("scaling")  => "scaling.e2e-hostrouting-ab12x.test"
//	f.Hostname("*")        => "*.e2e-hostrouting-ab12x.test"
func (f *Framework) Hostname(subdomain ...string) string {
	if len(subdomain) > 0 && subdomain[0] != "" {
		return subdomain[0] + "." + f.namespace + ".test"
	}
	return f.namespace + ".test"
}

// Get fetches a single Kubernetes object from the cluster.
func (f *Framework) Get(name, namespace string, obj k8s.Object) error {
	return f.client.Resources().Get(f.ctx, name, namespace, obj)
}

// createResource creates a single Kubernetes object in the cluster, failing the test on error.
func (f *Framework) createResource(obj k8s.Object) {
	f.t.Helper()
	if err := f.client.Resources().Create(f.ctx, obj); err != nil {
		f.t.Fatalf("failed to create %T %s/%s: %v", obj, obj.GetNamespace(), obj.GetName(), err)
	}
}

// DeleteResource removes a single Kubernetes object from the cluster, failing the test on error.
func (f *Framework) DeleteResource(obj k8s.Object) {
	f.t.Helper()
	if err := f.client.Resources().Delete(f.ctx, obj); err != nil {
		f.t.Fatalf("failed to delete %T %s/%s: %v", obj, obj.GetNamespace(), obj.GetName(), err)
	}
}

// updateResource updates a single Kubernetes object in the cluster, failing the test on error.
func (f *Framework) updateResource(obj k8s.Object) {
	f.t.Helper()
	if err := f.client.Resources().Update(f.ctx, obj); err != nil {
		f.t.Fatalf("failed to update %T %s/%s: %v", obj, obj.GetNamespace(), obj.GetName(), err)
	}
}
