package routing

import (
	"testing"

	kedav1alpha1 "github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/kubernetes/scheme"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	httpv1alpha1 "github.com/kedacore/http-add-on/operator/apis/http/v1alpha1"
)

func TestRouting(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Routing Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	By("bootstrapping test environment")
	// testEnv = &envtest.Environment{
	// 	CRDDirectoryPaths:     []string{filepath.Join("..", "..", "config", "crd", "bases")},
	// 	ErrorIfCRDPathMissing: true,
	// }

	var err error
	// cfg is defined in this file globally.
	// cfg, err = testEnv.Start()
	// Expect(err).NotTo(HaveOccurred())
	// Expect(cfg).NotTo(BeNil())

	err = httpv1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = kedav1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	//+kubebuilder:scaffold:scheme

	// k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	// Expect(err).NotTo(HaveOccurred())
	// Expect(k8sClient).NotTo(BeNil())
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	// err := testEnv.Stop()
	// Expect(err).NotTo(HaveOccurred())
})
