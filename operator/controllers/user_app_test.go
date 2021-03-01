package controllers

import (
	"time"

	"github.com/kedacore/http-add-on/operator/api/v1alpha1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("UserApp", func() {
	Context("Creating a user app", func() {
		var testInfra *commonTestInfra
		BeforeEach(func() {
			testInfra = newCommonTestInfra("testns", "testapp")
		})
		It("Should properly create a deployment and a service", func() {
			err := createUserApp(
				testInfra.ctx,
				testInfra.cfg,
				testInfra.cl,
				testInfra.logger,
				&testInfra.httpso,
			)
			Expect(err).To(BeNil())
			// make sure that httpso has the right conditions on it
			Expect(len(testInfra.httpso.Status.Conditions)).To(Equal(2))

			cond1 := testInfra.httpso.Status.Conditions[0]
			cond1ts, err := time.Parse(time.RFC3339, cond1.Timestamp)
			Expect(err).To(BeNil())
			Expect(time.Now().Sub(cond1ts) >= 0).To(BeTrue())
			Expect(cond1.Type).To(Equal(v1alpha1.Created))
			Expect(cond1.Status).To(Equal(metav1.ConditionTrue))
			Expect(cond1.Reason).To(Equal(v1alpha1.AppDeploymentCreated))

			cond2 := testInfra.httpso.Status.Conditions[1]
			cond2ts, err := time.Parse(time.RFC3339, cond2.Timestamp)
			Expect(err).To(BeNil())
			Expect(time.Now().Sub(cond2ts) >= 0).To(BeTrue())
			Expect(cond2.Type).To(Equal(v1alpha1.Created))
			Expect(cond2.Status).To(Equal(metav1.ConditionTrue))
			Expect(cond2.Reason).To(Equal(v1alpha1.AppServiceCreated))

			// check the deployment that was created
			deployment := &appsv1.Deployment{}
			err = testInfra.cl.Get(testInfra.ctx, client.ObjectKey{
				Name:      testInfra.cfg.Name,
				Namespace: testInfra.cfg.Namespace,
			}, deployment)
			Expect(err).To(BeNil())
			Expect(deployment.Name).To(Equal(testInfra.cfg.Name))
			Expect(len(deployment.Spec.Template.Spec.Containers)).To(Equal(1))
			Expect(deployment.Spec.Template.Spec.Containers[0].Image).To(Equal(testInfra.cfg.Image))

			// check the service that was created
			svc := &corev1.Service{}
			err = testInfra.cl.Get(testInfra.ctx, client.ObjectKey{
				Name:      testInfra.cfg.Name,
				Namespace: testInfra.cfg.Namespace,
			}, svc)
			Expect(err).To(BeNil())
			Expect(svc.Name).To(Equal(testInfra.cfg.Name))
			Expect(len(svc.Spec.Ports)).To(Equal(1))
			Expect(svc.Spec.Ports[0].Protocol).To(Equal(corev1.ProtocolTCP))
			Expect(svc.Spec.Ports[0].TargetPort.IntVal).To(Equal(testInfra.cfg.Port))
			Expect(svc.Spec.Ports[0].TargetPort.Type).To(Equal(intstr.Int))
			Expect(svc.Spec.Ports[0].Port).To(Equal(int32(8080)))
		})
		It("Should only create a service if an existing deployment is set", func() {
			testInfra.httpso.Spec.Image = ""
			testInfra.httpso.Spec.Deployment = &v1alpha1.DeploymentSpec{
				Name: "testExistingDeployment",
				Selector: map[string]string{
					"test": "selector",
				},
			}
			err := createUserApp(
				testInfra.ctx,
				testInfra.cfg,
				testInfra.cl,
				testInfra.logger,
				&testInfra.httpso,
			)
			Expect(err).To(BeNil())

			// make sure a deployment was not created
			deploymentList := &appsv1.DeploymentList{}
			err = testInfra.cl.List(testInfra.ctx, deploymentList)
			Expect(err).To(BeNil())
			Expect(len(deploymentList.Items)).To(Equal(0))

			// make sure a service was created
			service := &corev1.Service{}
			err = testInfra.cl.Get(testInfra.ctx, client.ObjectKey{
				Namespace: testInfra.cfg.Namespace,
				Name:      testInfra.cfg.Name,
			}, service)
			Expect(err).To(BeNil())
			// make sure that the port in the HTTPScaledObject was still recognized and set
			Expect(testInfra.cfg.Port).To(Equal(service.Spec.Ports[0].TargetPort.IntVal))
		})
	})
})
