// /*

// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at

//     http://www.apache.org/licenses/LICENSE-2.0

// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
// */

package http

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-logr/logr"
	kedav1alpha1 "github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	httpv1alpha1 "github.com/kedacore/http-add-on/operator/apis/http/v1alpha1"
	// +kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var cfg *rest.Config
var testEnv *envtest.Environment
var k8sClient client.Client

var ctx context.Context
var cancel context.CancelFunc

func TestControllers(t *testing.T) {
	RegisterFailHandler(Fail)
	suiteConfig, reporterConfig := GinkgoConfiguration()
	reporterConfig.JUnitReport = "functional-test-report.xml"
	RunSpecs(t, "Controllers Suite", suiteConfig, reporterConfig)
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
	}

	var err error
	done := make(chan interface{})
	go func() {
		defer GinkgoRecover()
		cfg, err = testEnv.Start()
		close(done)
	}()
	Eventually(done).WithTimeout(time.Minute).Should(BeClosed())
	Expect(err).ToNot(HaveOccurred())
	Expect(cfg).ToNot(BeNil())

	err = kedav1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = httpv1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	// +kubebuilder:scaffold:scheme

	ctx, cancel = context.WithCancel(context.Background())

	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme.Scheme,
	})
	Expect(err).ToNot(HaveOccurred())

	err = (&ClusterHTTPScalingSetReconciler{
		Client:        k8sManager.GetClient(),
		Scheme:        k8sManager.GetScheme(),
		KEDANamespace: "keda",
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	err = (&HTTPScalingSetReconciler{
		Client: k8sManager.GetClient(),
		Scheme: k8sManager.GetScheme(),
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).ToNot(HaveOccurred())
	Expect(k8sClient).ToNot(BeNil())

	createNamespace("keda")

	go func() {
		err = k8sManager.Start(ctx)
		Expect(err).ToNot(HaveOccurred())
	}()
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")

	// stop k8sManager
	cancel()

	err := testEnv.Stop()
	Expect(err).ToNot(HaveOccurred())
})

func validateResources(expected, value corev1.ResourceRequirements) {
	Expect(expected.Requests.Cpu().AsApproximateFloat64()).Should(BeEquivalentTo(value.Requests.Cpu().AsApproximateFloat64()))
	Expect(expected.Requests.Memory().AsApproximateFloat64()).Should(BeEquivalentTo(value.Requests.Memory().AsApproximateFloat64()))
	Expect(expected.Limits.Cpu().AsApproximateFloat64()).Should(BeEquivalentTo(value.Limits.Cpu().AsApproximateFloat64()))
	Expect(expected.Limits.Memory().AsApproximateFloat64()).Should(BeEquivalentTo(value.Limits.Memory().AsApproximateFloat64()))
}

func createNamespace(name string) {
	ns := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	err := k8sClient.Create(ctx, ns)
	Expect(err).ToNot(HaveOccurred())
}

type commonTestInfra struct {
	ns      string
	appName string
	ctx     context.Context
	cl      client.Client
	logger  logr.Logger
	httpso  httpv1alpha1.HTTPScaledObject
}

func newCommonTestInfra(namespace, appName string) *commonTestInfra {
	localScheme := runtime.NewScheme()
	utilruntime.Must(scheme.AddToScheme(localScheme))
	utilruntime.Must(httpv1alpha1.AddToScheme(localScheme))
	utilruntime.Must(kedav1alpha1.AddToScheme(localScheme))

	ctx := context.Background()
	cl := fake.NewClientBuilder().WithScheme(localScheme).Build()
	logger := logr.Discard()

	httpso := httpv1alpha1.HTTPScaledObject{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      appName,
			Labels: map[string]string{
				"label": "a",
			},
			Annotations: map[string]string{
				"annotation": "b",
			},
		},
		Spec: httpv1alpha1.HTTPScaledObjectSpec{
			ScaleTargetRef: httpv1alpha1.ScaleTargetRef{
				Service: appName,
				Port:    8081,
			},
			Hosts: []string{"myhost1.com", "myhost2.com"},
		},
	}

	return &commonTestInfra{
		ns:      namespace,
		appName: appName,
		ctx:     ctx,
		cl:      cl,
		logger:  logger,
		httpso:  httpso,
	}
}

func newCommonTestInfraWithSkipScaledObjectCreation(namespace, appName string) *commonTestInfra {
	localScheme := runtime.NewScheme()
	utilruntime.Must(scheme.AddToScheme(localScheme))
	utilruntime.Must(httpv1alpha1.AddToScheme(localScheme))
	utilruntime.Must(kedav1alpha1.AddToScheme(localScheme))

	ctx := context.Background()
	cl := fake.NewClientBuilder().WithScheme(localScheme).Build()
	logger := logr.Discard()

	httpso := httpv1alpha1.HTTPScaledObject{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      appName,
			Annotations: map[string]string{
				"httpscaledobject.keda.sh/skip-scaledobject-creation": "true",
			},
		},
		Spec: httpv1alpha1.HTTPScaledObjectSpec{
			ScaleTargetRef: httpv1alpha1.ScaleTargetRef{
				Name:    appName,
				Service: appName,
				Port:    8081,
			},
			Hosts: []string{"myhost1.com", "myhost2.com"},
		},
	}

	return &commonTestInfra{
		ns:      namespace,
		appName: appName,
		ctx:     ctx,
		cl:      cl,
		logger:  logger,
		httpso:  httpso,
	}
}
