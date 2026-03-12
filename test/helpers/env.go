//go:build e2e

package helpers

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"testing"
	"time"

	cmv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	kedav1alpha1 "github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	"golang.org/x/sync/errgroup"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apiserver/pkg/storage/names"
	"sigs.k8s.io/e2e-framework/klient/k8s/resources"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"

	httpv1beta1 "github.com/kedacore/http-add-on/operator/apis/http/v1beta1"
)

const (
	AddonNamespace          = "keda"
	e2eLabelKey             = "http.keda.sh/e2e"
	e2eLabelValue           = "true"
	interceptorDeployment   = "keda-add-ons-http-interceptor"
	interceptorProxyPort    = 8080
	interceptorProxyService = "keda-add-ons-http-interceptor-proxy"
	operatorDeployment      = "keda-add-ons-http-operator"
	scalerDeployment        = "keda-add-ons-http-scaler"
	scalerAddress           = "keda-add-ons-http-external-scaler.keda:9090"
)

var e2eLabels = labels.Set{e2eLabelKey: e2eLabelValue}

type testEnvConfig struct {
	proxyPort          int
	requiredNamespaces []string
}

type TestEnvOption func(*testEnvConfig)

// WithProxyPort sets the target port for the interceptor proxy.
func WithProxyPort(port int) TestEnvOption {
	return func(c *testEnvConfig) { c.proxyPort = port }
}

// WithRequiredNamespaces adds namespaces that must exist before tests run.
func WithRequiredNamespaces(namespaces ...string) TestEnvOption {
	return func(c *testEnvConfig) {
		c.requiredNamespaces = append(c.requiredNamespaces, namespaces...)
	}
}

func NewTestEnv(opts ...TestEnvOption) env.Environment {
	c := &testEnvConfig{proxyPort: interceptorProxyPort}
	for _, opt := range opts {
		opt(c)
	}

	cfg, err := envconf.NewFromFlags()
	if err != nil {
		log.Fatalf("failed to create test environment config: %v", err)
	}

	// Fail early if any required namespaces are missing.
	for _, ns := range c.requiredNamespaces {
		var nsObj corev1.Namespace
		if err := cfg.Client().Resources().Get(context.Background(), ns, "", &nsObj); err != nil {
			log.Fatalf("prerequisite namespace %q not found - is the dependency installed? (error: %v)", ns, err)
		}
	}

	// Wait for all HTTP add-on deployments to be rolled out before starting tests.
	// This prevents EOF errors from tests that start before the add-on is ready.
	var g errgroup.Group
	for _, name := range []string{operatorDeployment, interceptorDeployment, scalerDeployment} {
		g.Go(func() error {
			dep := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: AddonNamespace}}
			if err := wait.For(
				conditions.New(cfg.Client().Resources()).ResourceMatch(dep, deploymentRolledOut),
				wait.WithTimeout(defaultWaitTimeout),
			); err != nil {
				return fmt.Errorf("deployment %s/%s not ready: %w", AddonNamespace, name, err)
			}
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		log.Fatalf("add-on not ready: %v", err)
	}

	// Register all schemes we interact with in the E2E tests
	scheme := cfg.Client().Resources().GetScheme()
	if err := httpv1beta1.AddToScheme(scheme); err != nil {
		log.Fatalf("failed to register http-add-on scheme: %v", err)
	}
	if err := kedav1alpha1.AddToScheme(scheme); err != nil {
		log.Fatalf("failed to register KEDA scheme: %v", err)
	}
	if err := cmv1.AddToScheme(scheme); err != nil {
		log.Fatalf("failed to register cert-manager scheme: %v", err)
	}

	testenv := env.NewWithConfig(cfg)

	// Setup a shared socat proxy pod for the test suite.
	registerInterceptorProxy(testenv, c.proxyPort)

	// Cleanup all test resources
	registerCleanupOnSignal(cfg)

	testenv.BeforeEachTest(func(ctx context.Context, cfg *envconf.Config, t *testing.T) (context.Context, error) {
		ctx = contextWithClient(ctx, cfg.Client())

		// Create a randomized namespace per test
		testName := strings.ToLower(strings.TrimPrefix(t.Name(), "Test"))
		ns := randomName("e2e-" + testName)
		nsObj := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name:   ns,
				Labels: e2eLabels,
			},
		}
		if err := cfg.Client().Resources().Create(ctx, nsObj); err != nil {
			return ctx, fmt.Errorf("failed to create namespace %s: %w", ns, err)
		}
		log.Printf("created namespace %s for %s", ns, t.Name())

		return contextWithNamespace(ctx, ns), nil
	})

	testenv.AfterEachTest(func(ctx context.Context, cfg *envconf.Config, t *testing.T) (context.Context, error) {
		// Dump diagnostics for failed tests only in CI when we have no cluster access
		if t.Failed() && os.Getenv("CI") == "true" {
			dumpDiagnostics(ctx, t, cfg.Client())
		}

		// Cleanup the namespace created for this test
		ns := namespaceFromContext(ctx)
		nsObj := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}}
		if err := cfg.Client().Resources().Delete(ctx, nsObj); err != nil && !errors.IsNotFound(err) {
			log.Printf("warning: failed to delete namespace %s: %v", ns, err)
		}

		return ctx, nil
	})

	return testenv
}

// registerCleanupOnSignal spawns a goroutine that deletes all namespaces
// with the e2e label on interrupt (Ctrl+C), then exits.
func registerCleanupOnSignal(cfg *envconf.Config) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)

	go func() {
		sig := <-sigCh
		log.Printf("caught %s, cleaning up e2e namespaces...", sig)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		var nsList corev1.NamespaceList
		selector := resources.WithLabelSelector(labels.Set{e2eLabelKey: e2eLabelValue}.String())
		if err := cfg.Client().Resources().List(ctx, &nsList, selector); err != nil {
			log.Printf("failed to list e2e namespaces: %v", err)
			os.Exit(1)
		}

		for i := range nsList.Items {
			log.Printf("deleting namespace %s", nsList.Items[i].Name)
			if err := cfg.Client().Resources().Delete(ctx, &nsList.Items[i]); err != nil && !errors.IsNotFound(err) {
				log.Printf("failed to delete namespace %s: %v", nsList.Items[i].Name, err)
			}
		}

		log.Printf("cleaning up e2e cert-manager resources...")
		if err := cleanupCertManagerResources(ctx, cfg.Client()); err != nil {
			log.Printf("failed to clean up cert-manager resources: %v", err)
		}

		os.Exit(1)
	}()
}

// randomName generates a k8s-safe name from the given prefix.
func randomName(prefix string) string {
	return names.SimpleNameGenerator.GenerateName(prefix + "-")
}
