package http

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"

	"github.com/kedacore/http-add-on/operator/apis/http/v1alpha1"
)

var _ = Describe("ClusterHTTPScalingSetController", func() {
	Describe("functional tests", func() {
		It("ClusterHTTPScalingSet generates the interceptor and scaler using default values", func() {
			name := "default-values"
			interceptorName := fmt.Sprintf("%s-interceptor", name)
			interceptorProxyServiceName, interceptorAdminServiceName := getInterceptorServiceNames(name)

			scalerName := fmt.Sprintf("%s-external-scaler", name)
			scalerServiceName := fmt.Sprintf("%s-external-scaler", name)

			css := &v1alpha1.ClusterHTTPScalingSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: name,
				},
				Spec: v1alpha1.HTTPScalingSetSpec{
					Interceptor: v1alpha1.HTTPInterceptorSpec{},
					Scaler:      v1alpha1.HTTPScalerSpec{},
				},
			}
			err := k8sClient.Create(context.Background(), css)
			Expect(err).ToNot(HaveOccurred())

			// Validate interceptor proxy service
			interceptorProxyService := &corev1.Service{}
			Eventually(func() error {
				return k8sClient.Get(context.Background(), types.NamespacedName{Name: interceptorProxyServiceName, Namespace: "keda"}, interceptorProxyService)
			}).WithTimeout(30 * time.Second).WithPolling(5 * time.Second).ShouldNot(HaveOccurred())

			Expect(interceptorProxyService).ShouldNot(BeNil())
			Expect(interceptorProxyService.Spec.Selector).Should(HaveKeyWithValue("http.keda.sh/scaling-set", name))
			Expect(interceptorProxyService.Spec.Selector).Should(HaveKeyWithValue("http.keda.sh/scaling-set-component", "interceptor"))
			Expect(interceptorProxyService.Spec.Selector).Should(HaveKeyWithValue("http.keda.sh/scaling-set-kind", "ClusterHTTPScalingSet"))
			Expect(interceptorProxyService.Spec.Ports[0].TargetPort.StrVal).Should(Equal("proxy"))
			Expect(interceptorProxyService.Spec.Ports[0].Port).Should(Equal(int32(8080)))

			// Validate interceptor admin service
			interceptorAdminService := &corev1.Service{}
			Eventually(func() error {
				return k8sClient.Get(context.Background(), types.NamespacedName{Name: interceptorAdminServiceName, Namespace: "keda"}, interceptorAdminService)
			}).WithTimeout(30 * time.Second).WithPolling(5 * time.Second).ShouldNot(HaveOccurred())

			Expect(interceptorAdminService).ShouldNot(BeNil())
			Expect(interceptorAdminService.Spec.Selector).Should(HaveKeyWithValue("http.keda.sh/scaling-set", name))
			Expect(interceptorAdminService.Spec.Selector).Should(HaveKeyWithValue("http.keda.sh/scaling-set-component", "interceptor"))
			Expect(interceptorAdminService.Spec.Selector).Should(HaveKeyWithValue("http.keda.sh/scaling-set-kind", "ClusterHTTPScalingSet"))
			Expect(interceptorAdminService.Spec.Ports[0].TargetPort.StrVal).Should(Equal("admin"))
			Expect(interceptorAdminService.Spec.Ports[0].Port).Should(Equal(int32(9090)))

			// Validate interceptor deployment
			interceptorDeploy := &v1.Deployment{}
			Eventually(func() error {
				return k8sClient.Get(context.Background(), types.NamespacedName{Name: interceptorName, Namespace: "keda"}, interceptorDeploy)
			}).WithTimeout(30 * time.Second).WithPolling(5 * time.Second).ShouldNot(HaveOccurred())

			Expect(interceptorDeploy).ShouldNot(BeNil())
			Expect(interceptorDeploy.Spec.Selector.MatchLabels).Should(HaveKeyWithValue("http.keda.sh/scaling-set", name))
			Expect(interceptorDeploy.Spec.Selector.MatchLabels).Should(HaveKeyWithValue("http.keda.sh/scaling-set-component", "interceptor"))
			Expect(interceptorDeploy.Spec.Selector.MatchLabels).Should(HaveKeyWithValue("http.keda.sh/scaling-set-kind", "ClusterHTTPScalingSet"))
			Expect(*interceptorDeploy.Spec.Replicas).Should(Equal(int32(1)))
			Expect(interceptorDeploy.Spec.Template.Spec.Containers[0].Resources.Requests).Should(BeNil())
			Expect(interceptorDeploy.Spec.Template.Spec.Containers[0].Resources.Limits).Should(BeNil())

			// Validate scaler service
			scalerService := &corev1.Service{}
			Eventually(func() error {
				return k8sClient.Get(context.Background(), types.NamespacedName{Name: scalerServiceName, Namespace: "keda"}, scalerService)
			}).WithTimeout(30 * time.Second).WithPolling(5 * time.Second).ShouldNot(HaveOccurred())

			Expect(scalerService).ShouldNot(BeNil())
			Expect(scalerService.Spec.Selector).Should(HaveKeyWithValue("http.keda.sh/scaling-set", name))
			Expect(scalerService.Spec.Selector).Should(HaveKeyWithValue("http.keda.sh/scaling-set-component", "external-scaler"))
			Expect(scalerService.Spec.Selector).Should(HaveKeyWithValue("http.keda.sh/scaling-set-kind", "ClusterHTTPScalingSet"))
			Expect(scalerService.Spec.Ports[0].TargetPort.StrVal).Should(Equal("grpc"))
			Expect(scalerService.Spec.Ports[0].Port).Should(Equal(int32(9090)))

			// Validate scaler deployment
			scalerDeploy := &v1.Deployment{}
			Eventually(func() error {
				return k8sClient.Get(context.Background(), types.NamespacedName{Name: scalerName, Namespace: "keda"}, scalerDeploy)
			}).WithTimeout(30 * time.Second).WithPolling(5 * time.Second).ShouldNot(HaveOccurred())

			Expect(scalerDeploy).ShouldNot(BeNil())
			Expect(scalerDeploy.Spec.Selector.MatchLabels).Should(HaveKeyWithValue("http.keda.sh/scaling-set", name))
			Expect(scalerDeploy.Spec.Selector.MatchLabels).Should(HaveKeyWithValue("http.keda.sh/scaling-set-component", "external-scaler"))
			Expect(scalerDeploy.Spec.Selector.MatchLabels).Should(HaveKeyWithValue("http.keda.sh/scaling-set-kind", "ClusterHTTPScalingSet"))
			Expect(*scalerDeploy.Spec.Replicas).Should(Equal(int32(1)))
			Expect(scalerDeploy.Spec.Template.Spec.Containers[0].Resources.Requests).Should(BeNil())
			Expect(scalerDeploy.Spec.Template.Spec.Containers[0].Resources.Limits).Should(BeNil())
		})

		It("ClusterHTTPScalingSet generates the interceptor and scaler using custom values", func() {
			name := "custom-values"
			interceptorName := fmt.Sprintf("%s-interceptor", name)
			interceptorProxyServiceName, interceptorAdminServiceName := getInterceptorServiceNames(name)
			var interceptorReplicas int32 = 3
			interceptorResouces := corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					"cpu":    *resource.NewQuantity(10, resource.DecimalSI),
					"memory": *resource.NewQuantity(11, resource.DecimalSI),
				},
				Limits: corev1.ResourceList{
					"cpu":    *resource.NewQuantity(20, resource.DecimalSI),
					"memory": *resource.NewQuantity(21, resource.DecimalSI),
				},
			}
			var interceptorProxyPort int32 = 6666
			var interceptorAdminPort int32 = 7777

			scalerName := fmt.Sprintf("%s-external-scaler", name)
			scalerServiceName := fmt.Sprintf("%s-external-scaler", name)
			var scalerReplicas int32 = 2
			scalerResouces := corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					"cpu":    *resource.NewQuantity(30, resource.DecimalSI),
					"memory": *resource.NewQuantity(31, resource.DecimalSI),
				},
				Limits: corev1.ResourceList{
					"cpu":    *resource.NewQuantity(40, resource.DecimalSI),
					"memory": *resource.NewQuantity(41, resource.DecimalSI),
				},
			}
			var scalerPort int32 = 8888

			css := &v1alpha1.ClusterHTTPScalingSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: name,
				},
				Spec: v1alpha1.HTTPScalingSetSpec{
					Interceptor: v1alpha1.HTTPInterceptorSpec{
						Replicas:  ptr.To(interceptorReplicas),
						Resources: interceptorResouces,
						Config: &v1alpha1.HTTPInterceptorConfigurationSpec{
							ProxyPort: ptr.To(interceptorProxyPort),
							AdminPort: ptr.To(interceptorAdminPort),
						},
						Labels: map[string]string{
							"interceptor-label-1": "value-1",
						},
						Annotations: map[string]string{
							"interceptor-annotation-1": "value-2",
						},
					},
					Scaler: v1alpha1.HTTPScalerSpec{
						Replicas:  ptr.To(scalerReplicas),
						Resources: scalerResouces,
						Config: v1alpha1.HTTPScalerConfigurationSpec{
							Port: ptr.To(scalerPort),
						},
						Labels: map[string]string{
							"scaler-label-1": "value-3",
						},
						Annotations: map[string]string{
							"scaler-annotation-1": "value-4",
						},
					},
				},
			}
			err := k8sClient.Create(context.Background(), css)
			Expect(err).ToNot(HaveOccurred())

			// Validate interceptor proxy service
			interceptorProxyService := &corev1.Service{}
			Eventually(func() error {
				return k8sClient.Get(context.Background(), types.NamespacedName{Name: interceptorProxyServiceName, Namespace: "keda"}, interceptorProxyService)
			}).WithTimeout(30 * time.Second).WithPolling(5 * time.Second).ShouldNot(HaveOccurred())

			Expect(interceptorProxyService).ShouldNot(BeNil())
			Expect(interceptorProxyService.Spec.Selector).Should(HaveKeyWithValue("http.keda.sh/scaling-set", name))
			Expect(interceptorProxyService.Spec.Selector).Should(HaveKeyWithValue("http.keda.sh/scaling-set-component", "interceptor"))
			Expect(interceptorProxyService.Spec.Selector).Should(HaveKeyWithValue("http.keda.sh/scaling-set-kind", "ClusterHTTPScalingSet"))
			Expect(interceptorProxyService.Spec.Ports[0].TargetPort.StrVal).Should(Equal("proxy"))
			Expect(interceptorProxyService.Spec.Ports[0].Port).Should(Equal(interceptorProxyPort))

			// Validate interceptor admin service
			interceptorAdminService := &corev1.Service{}
			Eventually(func() error {
				return k8sClient.Get(context.Background(), types.NamespacedName{Name: interceptorAdminServiceName, Namespace: "keda"}, interceptorAdminService)
			}).WithTimeout(30 * time.Second).WithPolling(5 * time.Second).ShouldNot(HaveOccurred())

			Expect(interceptorAdminService).ShouldNot(BeNil())
			Expect(interceptorAdminService.Spec.Selector).Should(HaveKeyWithValue("http.keda.sh/scaling-set", name))
			Expect(interceptorAdminService.Spec.Selector).Should(HaveKeyWithValue("http.keda.sh/scaling-set-component", "interceptor"))
			Expect(interceptorAdminService.Spec.Selector).Should(HaveKeyWithValue("http.keda.sh/scaling-set-kind", "ClusterHTTPScalingSet"))
			Expect(interceptorAdminService.Spec.Ports[0].TargetPort.StrVal).Should(Equal("admin"))
			Expect(interceptorAdminService.Spec.Ports[0].Port).Should(Equal(interceptorAdminPort))

			// Validate interceptor deployment
			interceptorDeploy := &v1.Deployment{}
			Eventually(func() error {
				return k8sClient.Get(context.Background(), types.NamespacedName{Name: interceptorName, Namespace: "keda"}, interceptorDeploy)
			}).WithTimeout(30 * time.Second).WithPolling(5 * time.Second).ShouldNot(HaveOccurred())

			Expect(interceptorDeploy).ShouldNot(BeNil())
			Expect(interceptorDeploy.Spec.Selector.MatchLabels).Should(HaveKeyWithValue("http.keda.sh/scaling-set", name))
			Expect(interceptorDeploy.Spec.Selector.MatchLabels).Should(HaveKeyWithValue("http.keda.sh/scaling-set-component", "interceptor"))
			Expect(interceptorDeploy.Spec.Selector.MatchLabels).Should(HaveKeyWithValue("http.keda.sh/scaling-set-kind", "ClusterHTTPScalingSet"))
			Expect(*interceptorDeploy.Spec.Replicas).Should(Equal(interceptorReplicas))
			Expect(interceptorDeploy.ObjectMeta.Labels).Should(HaveKeyWithValue("interceptor-label-1", "value-1"))
			Expect(interceptorDeploy.ObjectMeta.Annotations).Should(HaveKeyWithValue("interceptor-annotation-1", "value-2"))
			validateResources(interceptorResouces, interceptorDeploy.Spec.Template.Spec.Containers[0].Resources)

			// Validate scaler service
			scalerService := &corev1.Service{}
			Eventually(func() error {
				return k8sClient.Get(context.Background(), types.NamespacedName{Name: scalerServiceName, Namespace: "keda"}, scalerService)
			}).WithTimeout(30 * time.Second).WithPolling(5 * time.Second).ShouldNot(HaveOccurred())

			Expect(scalerService).ShouldNot(BeNil())
			Expect(scalerService.Spec.Selector).Should(HaveKeyWithValue("http.keda.sh/scaling-set", name))
			Expect(scalerService.Spec.Selector).Should(HaveKeyWithValue("http.keda.sh/scaling-set-component", "external-scaler"))
			Expect(scalerService.Spec.Selector).Should(HaveKeyWithValue("http.keda.sh/scaling-set-kind", "ClusterHTTPScalingSet"))
			Expect(scalerService.Spec.Ports[0].TargetPort.StrVal).Should(Equal("grpc"))
			Expect(scalerService.Spec.Ports[0].Port).Should(Equal(scalerPort))

			// Validate scaler deployment
			scalerDeploy := &v1.Deployment{}
			Eventually(func() error {
				return k8sClient.Get(context.Background(), types.NamespacedName{Name: scalerName, Namespace: "keda"}, scalerDeploy)
			}).WithTimeout(30 * time.Second).WithPolling(5 * time.Second).ShouldNot(HaveOccurred())

			Expect(scalerDeploy).ShouldNot(BeNil())
			Expect(scalerDeploy.Spec.Selector.MatchLabels).Should(HaveKeyWithValue("http.keda.sh/scaling-set", name))
			Expect(scalerDeploy.Spec.Selector.MatchLabels).Should(HaveKeyWithValue("http.keda.sh/scaling-set-component", "external-scaler"))
			Expect(scalerDeploy.Spec.Selector.MatchLabels).Should(HaveKeyWithValue("http.keda.sh/scaling-set-kind", "ClusterHTTPScalingSet"))
			Expect(*scalerDeploy.Spec.Replicas).Should(Equal(scalerReplicas))
			Expect(scalerDeploy.ObjectMeta.Labels).Should(HaveKeyWithValue("scaler-label-1", "value-3"))
			Expect(scalerDeploy.ObjectMeta.Annotations).Should(HaveKeyWithValue("scaler-annotation-1", "value-4"))
			validateResources(scalerResouces, scalerDeploy.Spec.Template.Spec.Containers[0].Resources)
		})

	})
})
