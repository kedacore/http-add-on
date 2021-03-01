package controllers

import (
	"context"
	"time"

	logrtest "github.com/go-logr/logr/testing"
	"github.com/kedacore/http-add-on/operator/api/v1alpha1"
	"github.com/kedacore/http-add-on/operator/controllers/config"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("UserApp", func() {
	Context("Creating a ScaledObject", func() {
		It("Should properly create the ScaledObject for the user app", func() {
			const name = "testapp"
			const namespace = "testns"
			ctx := context.Background()
			cl := fake.NewFakeClient()
			cfg := config.AppInfo{
				App: config.App{
					Name:      name,
					Port:      8081,
					Image:     "arschles/testimg",
					Namespace: namespace,
				},
			}
			logger := logrtest.NullLogger{}
			httpso := &v1alpha1.HTTPScaledObject{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespace,
					Name:      name,
				},
				Spec: v1alpha1.HTTPScaledObjectSpec{
					AppName: name,
					Image:   "arschles/testapp",
					Port:    8081,
				},
			}
			err := createScaledObject(ctx, cfg, cl, logger, httpso)
			Expect(err).To(BeNil())
			// make sure that httpso has the right conditions on it
			Expect(len(httpso.Status.Conditions)).To(Equal(1))

			cond1 := httpso.Status.Conditions[0]
			cond1ts, err := time.Parse(time.RFC3339, cond1.Timestamp)
			Expect(err).To(BeNil())
			Expect(time.Now().Sub(cond1ts) >= 0).To(BeTrue())
			Expect(cond1.Type).To(Equal(v1alpha1.Created))
			Expect(cond1.Status).To(Equal(metav1.ConditionTrue))
			Expect(cond1.Reason).To(Equal(v1alpha1.ScaledObjectCreated))

			// check that the ScaledObject was created
			u := &unstructured.Unstructured{}
			u.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   "keda.sh",
				Kind:    "ScaledObject",
				Version: "v1alpha1",
			})
			err = cl.Get(ctx, client.ObjectKey{
				Namespace: cfg.App.Namespace,
				Name:      cfg.ScaledObjectName(),
			}, u)
			Expect(err).To(BeNil())
			metadataIface, found := u.Object["metadata"]
			metadata, ok := metadataIface.(map[string]interface{})
			Expect(found).To(BeTrue())
			Expect(ok).To(BeTrue())
			Expect(metadata["namespace"]).To(Equal(namespace))
			Expect(metadata["name"]).To(Equal(cfg.ScaledObjectName()))
			specIFace, found := u.Object["spec"]
			_, ok = specIFace.(map[string]interface{})
			Expect(found).To(BeTrue())
			Expect(ok).To(BeTrue())

		})
	})
})
