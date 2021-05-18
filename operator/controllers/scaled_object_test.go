package controllers

import (
	"fmt"
	"time"

	"github.com/kedacore/http-add-on/operator/api/v1alpha1"
	"github.com/kedacore/http-add-on/operator/controllers/config"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("UserApp", func() {
	Context("Creating a ScaledObject", func() {
		const externalScalerHostName = "mysvc.myns.svc.cluster.local:9090"

		var testInfra *commonTestInfra
		BeforeEach(func() {
			testInfra = newCommonTestInfra("testns", "testapp")
		})
		It("Should properly create the ScaledObject for the user app", func() {
			err := createScaledObjects(
				testInfra.ctx,
				testInfra.cfg,
				testInfra.cl,
				testInfra.logger,
				externalScalerHostName,
				&testInfra.httpso,
			)
			Expect(err).To(BeNil())

			// make sure that httpso has the right conditions on it:
			//
			// - AppScaledObjectCreated
			// - InterceptorScaledObjectCreated
			Expect(len(testInfra.httpso.Status.Conditions)).To(Equal(2))

			cond1 := testInfra.httpso.Status.Conditions[0]
			cond1ts, err := time.Parse(time.RFC3339, cond1.Timestamp)
			Expect(err).To(BeNil())
			Expect(time.Since(cond1ts) >= 0).To(BeTrue())
			Expect(cond1.Type).To(Equal(v1alpha1.Created))
			Expect(cond1.Status).To(Equal(metav1.ConditionTrue))
			Expect(cond1.Reason).To(Equal(v1alpha1.AppScaledObjectCreated))

			cond2 := testInfra.httpso.Status.Conditions[1]
			cond2ts, err := time.Parse(time.RFC3339, cond2.Timestamp)
			Expect(err).To(BeNil())
			Expect(time.Since(cond2ts) >= 0).To(BeTrue())
			Expect(cond2.Type).To(Equal(v1alpha1.Created))
			Expect(cond2.Status).To(Equal(metav1.ConditionTrue))
			Expect(cond2.Reason).To(Equal(v1alpha1.InterceptorScaledObjectCreated))

			// check that the app ScaledObject was created
			u := &unstructured.Unstructured{}
			u.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   "keda.sh",
				Kind:    "ScaledObject",
				Version: "v1alpha1",
			})
			objectKey := client.ObjectKey{
				Namespace: testInfra.cfg.Namespace,
				Name:      config.AppScaledObjectName(&testInfra.httpso),
			}
			err = testInfra.cl.Get(testInfra.ctx, objectKey, u)
			Expect(err).To(BeNil())

			metadata, err := getKeyAsMap(u.Object, "metadata")
			Expect(err).To(BeNil())
			Expect(metadata["namespace"]).To(Equal(testInfra.ns))
			Expect(metadata["name"]).To(Equal(config.AppScaledObjectName(&testInfra.httpso)))

			spec, err := getKeyAsMap(u.Object, "spec")
			Expect(err).To(BeNil())
			Expect(spec["minReplicaCount"]).To(BeNumerically("==", testInfra.httpso.Spec.Replicas.Min))
			Expect(spec["maxReplicaCount"]).To(BeNumerically("==", testInfra.httpso.Spec.Replicas.Max))

			// check that the interceptor ScaledObject was created

			objectKey.Name = config.InterceptorScaledObjectName(&testInfra.httpso)
			err = testInfra.cl.Get(testInfra.ctx, objectKey, u)
			Expect(err).To(BeNil())

			metadata, err = getKeyAsMap(u.Object, "metadata")
			Expect(err).To(BeNil())
			Expect(metadata["namespace"]).To(Equal(testInfra.ns))
			Expect(metadata["name"]).To(Equal(config.InterceptorScaledObjectName(&testInfra.httpso)))

			spec, err = getKeyAsMap(u.Object, "spec")
			Expect(err).To(BeNil())
			Expect(spec["minReplicaCount"]).To(BeNumerically("==", testInfra.httpso.Spec.Replicas.Min))
			Expect(spec["maxReplicaCount"]).To(BeNumerically("==", testInfra.httpso.Spec.Replicas.Max))
		})
	})
})

func getKeyAsMap(m map[string]interface{}, key string) (map[string]interface{}, error) {
	iface, ok := m[key]
	if !ok {
		return nil, fmt.Errorf("key %s not found in map", key)
	}
	val, ok := iface.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("key %s was not a map[string]interface{}", key)
	}
	return val, nil

}
