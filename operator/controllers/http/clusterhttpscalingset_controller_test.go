package http

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/kedacore/http-add-on/operator/apis/http/v1alpha1"
)

var _ = Describe("ClusterHTTPScalingSetController", func() {

	// var (
	// 	testLogger = zap.New(zap.UseDevMode(true), zap.WriteTo(GinkgoWriter))
	// )

	Describe("functional tests", func() {
		It("ClusterHTTPScalingSet generates the interceptor and scaler", func() {
			name := "testing-name"
			interceptorName := fmt.Sprintf("%s-interceptor", name)
			interceptorProxyServiceName, interceptorAdminServiceName := getInterceptorServiceNames(name)
			scalerName := fmt.Sprintf("%s-external-scaler", name)
			scalerServiceName := fmt.Sprintf("%s-external-scaler", name)

			css := &v1alpha1.ClusterHTTPScalingSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: name,
				},
				Spec: v1alpha1.HTTPScalingSetSpec{
					Interceptor: v1alpha1.HTTPInterceptorSepc{},
					Scaler:      v1alpha1.HTTPScalerSepc{},
				},
			}
			err := k8sClient.Create(context.Background(), css)
			Expect(err).ToNot(HaveOccurred())

			interceptorProxyService := &corev1.Service{}
			Eventually(func() error {
				return k8sClient.Get(context.Background(), types.NamespacedName{Name: interceptorProxyServiceName, Namespace: "keda"}, interceptorProxyService)
			}).WithTimeout(30 * time.Second).WithPolling(5 * time.Second).ShouldNot(HaveOccurred())

			Expect(interceptorProxyService).ShouldNot(BeNil())
			Expect(interceptorProxyService.Spec.Selector).Should(HaveKeyWithValue("http.keda.sh/scaling-set", name))
			Expect(interceptorProxyService.Spec.Selector).Should(HaveKeyWithValue("http.keda.sh/scaling-set-component", "interceptor"))
			Expect(interceptorProxyService.Spec.Selector).Should(HaveKeyWithValue("http.keda.sh/scaling-set-kind", "ClusterHTTPScalingSet"))

			interceptorAdminService := &corev1.Service{}
			Eventually(func() error {
				return k8sClient.Get(context.Background(), types.NamespacedName{Name: interceptorAdminServiceName, Namespace: "keda"}, interceptorAdminService)
			}).WithTimeout(30 * time.Second).WithPolling(5 * time.Second).ShouldNot(HaveOccurred())

			Expect(interceptorAdminService).ShouldNot(BeNil())
			Expect(interceptorAdminService.Spec.Selector).Should(HaveKeyWithValue("http.keda.sh/scaling-set", name))
			Expect(interceptorAdminService.Spec.Selector).Should(HaveKeyWithValue("http.keda.sh/scaling-set-component", "interceptor"))
			Expect(interceptorAdminService.Spec.Selector).Should(HaveKeyWithValue("http.keda.sh/scaling-set-kind", "ClusterHTTPScalingSet"))

			interceptorDeploy := &v1.Deployment{}
			Eventually(func() error {
				return k8sClient.Get(context.Background(), types.NamespacedName{Name: interceptorName, Namespace: "keda"}, interceptorDeploy)
			}).WithTimeout(30 * time.Second).WithPolling(5 * time.Second).ShouldNot(HaveOccurred())

			Expect(interceptorDeploy).ShouldNot(BeNil())
			Expect(interceptorDeploy.Spec.Selector.MatchLabels).Should(HaveKeyWithValue("http.keda.sh/scaling-set", name))
			Expect(interceptorDeploy.Spec.Selector.MatchLabels).Should(HaveKeyWithValue("http.keda.sh/scaling-set-component", "interceptor"))
			Expect(interceptorDeploy.Spec.Selector.MatchLabels).Should(HaveKeyWithValue("http.keda.sh/scaling-set-kind", "ClusterHTTPScalingSet"))

			scalerService := &corev1.Service{}
			Eventually(func() error {
				return k8sClient.Get(context.Background(), types.NamespacedName{Name: scalerServiceName, Namespace: "keda"}, scalerService)
			}).WithTimeout(30 * time.Second).WithPolling(5 * time.Second).ShouldNot(HaveOccurred())

			Expect(scalerService).ShouldNot(BeNil())
			Expect(scalerService.Spec.Selector).Should(HaveKeyWithValue("http.keda.sh/scaling-set", name))
			Expect(scalerService.Spec.Selector).Should(HaveKeyWithValue("http.keda.sh/scaling-set-component", "external-scaler"))
			Expect(scalerService.Spec.Selector).Should(HaveKeyWithValue("http.keda.sh/scaling-set-kind", "ClusterHTTPScalingSet"))

			scalerDeploy := &v1.Deployment{}
			Eventually(func() error {
				return k8sClient.Get(context.Background(), types.NamespacedName{Name: scalerName, Namespace: "keda"}, scalerDeploy)
			}).WithTimeout(30 * time.Second).WithPolling(5 * time.Second).ShouldNot(HaveOccurred())

			Expect(scalerDeploy).ShouldNot(BeNil())
			Expect(scalerDeploy.Spec.Selector.MatchLabels).Should(HaveKeyWithValue("http.keda.sh/scaling-set", name))
			Expect(scalerDeploy.Spec.Selector.MatchLabels).Should(HaveKeyWithValue("http.keda.sh/scaling-set-component", "external-scaler"))
			Expect(scalerDeploy.Spec.Selector.MatchLabels).Should(HaveKeyWithValue("http.keda.sh/scaling-set-kind", "ClusterHTTPScalingSet"))
		})
	})
})
