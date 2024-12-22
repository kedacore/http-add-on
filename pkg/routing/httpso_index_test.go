package routing

import (
	httpv1alpha1 "github.com/kedacore/http-add-on/operator/apis/http/v1alpha1"
	"github.com/kedacore/http-add-on/pkg/k8s"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("httpSOIndex", func() {
	var (
		httpso0 = &httpv1alpha1.HTTPScaledObject{
			ObjectMeta: metav1.ObjectMeta{
				Name: "keda-sh",
			},
			Spec: httpv1alpha1.HTTPScaledObjectSpec{
				Hosts: []string{
					"keda.sh",
				},
			},
		}

		httpso0NamespacedName = k8s.NamespacedNameFromObject(httpso0)
		httpso0IndexKey       = newTableMemoryIndexKey(httpso0NamespacedName)

		httpso1 = &httpv1alpha1.HTTPScaledObject{
			ObjectMeta: metav1.ObjectMeta{
				Name: "one-one-one-one",
			},
			Spec: httpv1alpha1.HTTPScaledObjectSpec{
				Hosts: []string{
					"1.1.1.1",
				},
			},
		}
		httpso1NamespacedName = k8s.NamespacedNameFromObject(httpso1)
		httpso1IndexKey       = newTableMemoryIndexKey(httpso1NamespacedName)
	)
	Context("New", func() {
		It("returns a httpSOIndex with initialized tree", func() {
			index := newHTTPSOIndex()
			Expect(index.radix).NotTo(BeNil())
		})
	})

	Context("Get / Insert", func() {
		It("Get on empty httpSOIndex returns nil", func() {
			index := newHTTPSOIndex()
			_, ok := index.get(httpso0IndexKey)
			Expect(ok).To(BeFalse())
		})
		It("httpSOIndex insert will return previous object if set", func() {
			index := newHTTPSOIndex()
			index, prevVal, prevSet := index.insert(httpso0IndexKey, httpso0)
			Expect(prevSet).To(BeFalse())
			Expect(prevVal).To(BeNil())
			httpso0Copy := httpso0.DeepCopy()
			httpso0Copy.Name = "httpso0Copy"
			index, prevVal, prevSet = index.insert(httpso0IndexKey, httpso0Copy)
			Expect(prevSet).To(BeTrue())
			Expect(prevVal).To(Equal(httpso0))
			Expect(prevVal).ToNot(Equal(httpso0Copy))
			httpso, ok := index.get(httpso0IndexKey)
			Expect(ok).To(BeTrue())
			Expect(httpso).ToNot(Equal(httpso0))
			Expect(httpso).To(Equal(httpso0Copy))
		})

		It("httpSOIndex with new object inserted returns object", func() {
			index := newHTTPSOIndex()
			index, httpso, prevSet := index.insert(httpso0IndexKey, httpso0)
			Expect(prevSet).To(BeFalse())
			Expect(httpso).To(BeNil())
			httpso, ok := index.get(httpso0IndexKey)
			Expect(ok).To(BeTrue())
			Expect(httpso).To(Equal(httpso0))
		})

		It("httpSOIndex with new object inserted retains other object", func() {
			index := newHTTPSOIndex()

			index, _, _ = index.insert(httpso0IndexKey, httpso0)
			httpso, ok := index.get(httpso0IndexKey)
			Expect(ok).To(BeTrue())
			Expect(httpso).To(Equal(httpso0))

			_, ok = index.get(httpso1IndexKey)
			Expect(ok).To(BeFalse())

			index, _, _ = index.insert(httpso1IndexKey, httpso1)
			httpso, ok = index.get(httpso1IndexKey)
			Expect(ok).To(BeTrue())
			Expect(httpso).To(Equal(httpso1))

			// httpso0 still there
			httpso, ok = index.get(httpso0IndexKey)
			Expect(ok).To(BeTrue())
			Expect(httpso).To(Equal(httpso0))
		})
	})

	Context("Get / Delete", func() {
		It("delete on empty httpSOIndex returns nil", func() {
			index := newHTTPSOIndex()
			_, httpso, oldSet := index.delete(httpso0IndexKey)
			Expect(httpso).To(BeNil())
			Expect(oldSet).To(BeFalse())
		})

		It("double delete returns nil the second time", func() {
			index := newHTTPSOIndex()
			index, _, _ = index.insert(httpso0IndexKey, httpso0)
			index, _, _ = index.insert(httpso1IndexKey, httpso1)
			index, deletedVal, oldSet := index.delete(httpso0IndexKey)
			Expect(deletedVal).To(Equal(httpso0))
			Expect(oldSet).To(BeTrue())
			index, deletedVal, oldSet = index.delete(httpso0IndexKey)
			Expect(deletedVal).To(BeNil())
			Expect(oldSet).To(BeFalse())
		})

		It("delete on httpSOIndex removes object ", func() {
			index := newHTTPSOIndex()
			index, _, _ = index.insert(httpso0IndexKey, httpso0)
			httpso, ok := index.get(httpso0IndexKey)
			Expect(ok).To(BeTrue())
			Expect(httpso).To(Equal(httpso0))
			index, deletedVal, oldSet := index.delete(httpso0IndexKey)
			Expect(deletedVal).To(Equal(httpso0))
			Expect(oldSet).To(BeTrue())
			httpso, ok = index.get(httpso0IndexKey)
			Expect(httpso).To(BeNil())
			Expect(ok).To(BeFalse())
		})

		It("httpSOIndex delete on one object does not affect other", func() {
			index := newHTTPSOIndex()

			index, _, _ = index.insert(httpso0IndexKey, httpso0)
			index, _, _ = index.insert(httpso1IndexKey, httpso1)
			httpso, ok := index.get(httpso0IndexKey)
			Expect(ok).To(BeTrue())
			Expect(httpso).To(Equal(httpso0))
			index, deletedVal, oldSet := index.delete(httpso1IndexKey)
			Expect(deletedVal).To(Equal(httpso1))
			Expect(oldSet).To(BeTrue())
			httpso, ok = index.get(httpso0IndexKey)
			Expect(ok).To(BeTrue())
			Expect(httpso).To(Equal(httpso0))
			httpso, ok = index.get(httpso1IndexKey)
			Expect(ok).To(BeFalse())
			Expect(httpso).To(BeNil())
		})
	})
})
