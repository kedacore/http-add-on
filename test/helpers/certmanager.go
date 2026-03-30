//go:build e2e

package helpers

import (
	"context"
	"crypto/x509"
	"fmt"

	cmv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	cmmeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/e2e-framework/klient"
	"sigs.k8s.io/e2e-framework/klient/k8s"
	"sigs.k8s.io/e2e-framework/klient/k8s/resources"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

const CertManagerNamespace = "cert-manager"

// SetupCAHierarchy sets up a cert-manager CA hierarchy and stores the CA issuer
// and CA pool in context for use by certificate helpers.
func SetupCAHierarchy(testenv env.Environment) {
	testenv.Setup(func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
		client := cfg.Client()

		// Create a self-signed issuer required for creating the CA cert
		bootstrapIssuer := &cmv1.ClusterIssuer{
			ObjectMeta: metav1.ObjectMeta{
				Name:   randomName("e2e-bootstrap"),
				Labels: e2eLabels,
			},
			Spec: cmv1.IssuerSpec{
				IssuerConfig: cmv1.IssuerConfig{
					SelfSigned: &cmv1.SelfSignedIssuer{},
				},
			},
		}
		if err := client.Resources().Create(ctx, bootstrapIssuer); err != nil {
			return ctx, fmt.Errorf("failed to create self-signed ClusterIssuer: %w", err)
		}

		// Create the root certificate for our CA
		caCertName := randomName("e2e-ca")
		caCert := &cmv1.Certificate{
			ObjectMeta: metav1.ObjectMeta{
				Name:      caCertName,
				Namespace: CertManagerNamespace,
				Labels:    e2eLabels,
			},
			Spec: cmv1.CertificateSpec{
				CommonName: caCertName,
				IsCA:       true,
				SecretName: caCertName,
				IssuerRef: cmmeta.ObjectReference{
					Name:  bootstrapIssuer.Name,
					Kind:  cmv1.ClusterIssuerKind,
					Group: cmv1.SchemeGroupVersion.Group,
				},
			},
		}
		if err := client.Resources().Create(ctx, caCert); err != nil {
			return ctx, fmt.Errorf("failed to create CA Certificate: %w", err)
		}

		// Create the actual ClusterIssuer that signs all test certificates
		caIssuer := &cmv1.ClusterIssuer{
			ObjectMeta: metav1.ObjectMeta{
				Name:   randomName("e2e-ca-issuer"),
				Labels: e2eLabels,
			},
			Spec: cmv1.IssuerSpec{
				IssuerConfig: cmv1.IssuerConfig{
					CA: &cmv1.CAIssuer{
						SecretName: caCert.Spec.SecretName,
					},
				},
			},
		}
		if err := client.Resources().Create(ctx, caIssuer); err != nil {
			return ctx, fmt.Errorf("failed to create CA ClusterIssuer: %w", err)
		}

		// Extract the root CA cert for usage in our HTTP clients TLS Config
		caSecret, err := waitForCertificateSecret(client, caCert.Namespace, caCert.Spec.SecretName)
		if err != nil {
			return ctx, fmt.Errorf("CA Secret not ready: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(caSecret.Data["ca.crt"]) {
			return ctx, fmt.Errorf("failed to parse CA cert from Secret %s/%s", caCert.Namespace, caCert.Spec.SecretName)
		}

		ctx = contextWithCAIssuer(ctx, caIssuer.Name)
		ctx = contextWithCAPool(ctx, pool)
		return ctx, nil
	})

	testenv.Finish(func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
		if err := cleanupCertManagerResources(ctx, cfg.Client()); err != nil {
			return ctx, fmt.Errorf("failed to cleanup cert-manager resources: %w", err)
		}
		return ctx, nil
	})
}

// CreateCertificate creates a cert-manager Certificate for the given DNS names
// and returns the secret name.
func (f *Framework) CreateCertificate(dnsNames []string) string {
	f.t.Helper()
	certName, err := createCertificate(f.ctx, f.client, f.namespace, f.caIssuer, dnsNames)
	if err != nil {
		f.t.Fatalf("failed to issue certificate: %v", err)
	}
	return certName
}

func createCertificate(ctx context.Context, client klient.Client, namespace, caIssuer string, dnsNames []string) (string, error) {
	certName := randomName("e2e-cert")
	cert := &cmv1.Certificate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      certName,
			Namespace: namespace,
			Labels:    e2eLabels,
		},
		Spec: cmv1.CertificateSpec{
			SecretName: certName,
			DNSNames:   dnsNames,
			IssuerRef: cmmeta.ObjectReference{
				Name:  caIssuer,
				Kind:  cmv1.ClusterIssuerKind,
				Group: cmv1.SchemeGroupVersion.Group,
			},
		},
	}
	if err := client.Resources().Create(ctx, cert); err != nil {
		return "", fmt.Errorf("failed to create Certificate %s/%s: %w", namespace, certName, err)
	}

	if _, err := waitForCertificateSecret(client, namespace, certName); err != nil {
		return "", fmt.Errorf("certificate Secret %s/%s not ready: %w", namespace, certName, err)
	}
	return certName, nil
}

// waitForCertificateSecret waits for a cert-manager Secret to be populated with the certificate.
func waitForCertificateSecret(client klient.Client, namespace, name string) (*corev1.Secret, error) {
	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}}
	if err := wait.For(
		conditions.New(client.Resources()).ResourceMatch(secret, func(object k8s.Object) bool {
			s := object.(*corev1.Secret)
			// tls.crt etc are populated after the secret is created, wait for these fields
			return len(s.Data["tls.crt"]) > 0 && len(s.Data["tls.key"]) > 0
		}),
		wait.WithTimeout(defaultWaitTimeout),
	); err != nil {
		return nil, err
	}
	return secret, nil
}

// cleanupCertManagerResources deletes all cert-manager ClusterIssuers, Certificates,
// and their generated Secrets with the e2e label.
func cleanupCertManagerResources(ctx context.Context, client klient.Client) error {
	selector := resources.WithLabelSelector(labels.Set{e2eLabelKey: e2eLabelValue}.String())

	var issuers cmv1.ClusterIssuerList
	if err := client.Resources().List(ctx, &issuers, selector); err != nil {
		return fmt.Errorf("failed to list ClusterIssuers: %w", err)
	}
	for i := range issuers.Items {
		if err := client.Resources().Delete(ctx, &issuers.Items[i]); err != nil && !errors.IsNotFound(err) {
			return fmt.Errorf("failed to delete ClusterIssuer %s: %w", issuers.Items[i].Name, err)
		}
	}

	// Clean up Certificates and their Secrets in both the addon and cert-manager namespaces.
	for _, ns := range []string{AddonNamespace, CertManagerNamespace} {
		var certs cmv1.CertificateList
		if err := client.Resources().WithNamespace(ns).List(ctx, &certs, selector); err != nil {
			return fmt.Errorf("failed to list Certificates in %s: %w", ns, err)
		}
		for i := range certs.Items {
			if certs.Items[i].Spec.SecretName != "" {
				secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
					Name:      certs.Items[i].Spec.SecretName,
					Namespace: ns,
				}}
				if err := client.Resources().Delete(ctx, secret); err != nil && !errors.IsNotFound(err) {
					return fmt.Errorf("failed to delete Secret %s/%s: %w", ns, certs.Items[i].Spec.SecretName, err)
				}
			}
			if err := client.Resources().Delete(ctx, &certs.Items[i]); err != nil && !errors.IsNotFound(err) {
				return fmt.Errorf("failed to delete Certificate %s/%s: %w", ns, certs.Items[i].Name, err)
			}
		}
	}

	return nil
}
