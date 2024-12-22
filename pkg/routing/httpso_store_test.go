package routing

import (
	httpv1alpha1 "github.com/kedacore/http-add-on/operator/apis/http/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("httpSOStore", func() {
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
		httpso0List = &httpv1alpha1.HTTPScaledObjectList{
			Items: []*httpv1alpha1.HTTPScaledObject{
				httpso0,
			},
		}

		httpso0StoreKeys = NewKeysFromHTTPSO(httpso0)
		httpso1          = &httpv1alpha1.HTTPScaledObject{
			ObjectMeta: metav1.ObjectMeta{
				Name: "one-one-one-one",
			},
			Spec: httpv1alpha1.HTTPScaledObjectSpec{
				Hosts: []string{
					"1.1.1.1",
				},
			},
		}
		httpso1List = &httpv1alpha1.HTTPScaledObjectList{
			Items: []*httpv1alpha1.HTTPScaledObject{
				httpso1,
			},
		}
		httpso1StoreKeys = NewKeysFromHTTPSO(httpso1)
	)
	Context("New", func() {
		It("returns a httpSOStore with initialized tree", func() {
			store := newHTTPSOStore()
			Expect(store.radix).NotTo(BeNil())
		})
	})

	Context("Get / Insert", func() {
		It("Get on empty httpSOStore returns nil", func() {
			store := newHTTPSOStore()
			for _, key := range httpso0StoreKeys {
				_, ok := store.get(key)
				Expect(ok).To(BeFalse())
			}
		})

		It("httpSOStore with new object inserted returns old object", func() {
			store := newHTTPSOStore()
			for _, key := range httpso0StoreKeys {
				var prevVal *httpv1alpha1.HTTPScaledObjectList
				var prevSet bool
				store, prevVal, prevSet = store.insert(key, httpso0List)
				Expect(prevVal).To(BeNil())
				Expect(prevSet).To(BeFalse())
			}
			httpso0ListCopy := &httpv1alpha1.HTTPScaledObjectList{
				Items: httpso0List.Items,
				ListMeta: metav1.ListMeta{
					ResourceVersion: "httpso0ListCopy",
				},
			}
			for _, key := range httpso0StoreKeys {
				var prevVal *httpv1alpha1.HTTPScaledObjectList
				var prevSet bool
				store, prevVal, prevSet = store.insert(key, httpso0ListCopy)
				Expect(prevVal).To(Equal(httpso0List))
				Expect(prevVal).ToNot(Equal(httpso0ListCopy))
				Expect(prevSet).To(BeTrue())
			}
		})

		It("httpSOStore insert will return object if set", func() {
			store := newHTTPSOStore()
			for _, key := range httpso0StoreKeys {
				var prevVal *httpv1alpha1.HTTPScaledObjectList
				var prevSet bool
				store, prevVal, prevSet = store.insert(key, httpso0List)
				Expect(prevVal).To(BeNil())
				Expect(prevSet).To(BeFalse())
			}
			for _, key := range httpso0StoreKeys {
				httpsoList, ok := store.get(key)
				Expect(httpsoList).To(Equal(httpso0List))
				Expect(ok).To(BeTrue())
			}
		})

		It("httpSOStore with new object inserted retains other object", func() {
			store := newHTTPSOStore()
			for _, key := range httpso0StoreKeys {
				store, _, _ = store.insert(key, httpso0List)
			}

			for _, key := range httpso0StoreKeys {
				httpsoList, ok := store.get(key)
				Expect(ok).To(BeTrue())
				Expect(httpsoList).To(Equal(httpso0List))
			}

			for _, key := range httpso1StoreKeys {
				httpsoList, ok := store.get(key)
				Expect(ok).To(BeFalse())
				Expect(httpsoList).To(BeNil())
			}

			for _, key := range httpso1StoreKeys {
				store, _, _ = store.insert(key, httpso1List)
			}

			for _, key := range httpso1StoreKeys {
				httpsoList, ok := store.get(key)
				Expect(ok).To(BeTrue())
				Expect(httpsoList).To(Equal(httpso1List))
			}

			for _, key := range httpso0StoreKeys {
				httpsoList, ok := store.get(key)
				Expect(ok).To(BeTrue())
				Expect(httpsoList).To(Equal(httpso0List))
			}
		})
	})

	Context("Get / Delete", func() {
		It("delete on empty httpSOStore returns nil", func() {
			store := newHTTPSOStore()
			for _, key := range httpso0StoreKeys {
				var prevVal *httpv1alpha1.HTTPScaledObjectList
				var prevSet bool
				store, prevVal, prevSet = store.delete(key)
				Expect(prevVal).To(BeNil())
				Expect(prevSet).To(BeFalse())
			}
		})

		It("double delete returns nil the second time", func() {
			store := newHTTPSOStore()
			for _, key := range httpso0StoreKeys {
				store, _, _ = store.insert(key, httpso0List)
			}
			for _, key := range httpso1StoreKeys {
				store, _, _ = store.insert(key, httpso1List)
			}
			for _, key := range httpso0StoreKeys {
				var prevVal *httpv1alpha1.HTTPScaledObjectList
				var prevSet bool
				store, prevVal, prevSet = store.delete(key)
				Expect(prevVal).To(Equal(httpso0List))
				Expect(prevSet).To(BeTrue())
			}
			for _, key := range httpso0StoreKeys {
				var prevVal *httpv1alpha1.HTTPScaledObjectList
				var prevSet bool
				store, prevVal, prevSet = store.delete(key)
				Expect(prevVal).To(BeNil())
				Expect(prevSet).To(BeFalse())
			}
		})

		It("delete on httpSOStore removes object ", func() {
			store := newHTTPSOStore()
			for _, key := range httpso0StoreKeys {
				store, _, _ = store.insert(key, httpso0List)
			}
			for _, key := range httpso0StoreKeys {
				httpsoList, ok := store.get(key)
				Expect(ok).To(BeTrue())
				Expect(httpsoList).To(Equal(httpso0List))
			}

			for _, key := range httpso0StoreKeys {
				var prevVal *httpv1alpha1.HTTPScaledObjectList
				var prevSet bool
				store, prevVal, prevSet = store.delete(key)
				Expect(prevVal).To(Equal(httpso0List))
				Expect(prevSet).To(BeTrue())
			}
			for _, key := range httpso0StoreKeys {
				httpsoList, ok := store.get(key)
				Expect(httpsoList).To(BeNil())
				Expect(ok).To(BeFalse())
			}
		})

		It("httpSOStore delete on one object does not affect other", func() {
			store := newHTTPSOStore()
			for _, key := range httpso0StoreKeys {
				store, _, _ = store.insert(key, httpso0List)
			}
			for _, key := range httpso1StoreKeys {
				store, _, _ = store.insert(key, httpso1List)
			}
			for _, key := range httpso0StoreKeys {
				httpsoList, ok := store.get(key)
				Expect(ok).To(BeTrue())
				Expect(httpsoList).To(Equal(httpso0List))
			}
			for _, key := range httpso1StoreKeys {
				var prevVal *httpv1alpha1.HTTPScaledObjectList
				var prevSet bool
				store, prevVal, prevSet = store.delete(key)
				Expect(prevVal).To(Equal(httpso1List))
				Expect(prevSet).To(BeTrue())
			}

			for _, key := range httpso0StoreKeys {
				httpsoList, ok := store.get(key)
				Expect(ok).To(BeTrue())
				Expect(httpsoList).To(Equal(httpso0List))
			}
			for _, key := range httpso1StoreKeys {
				httpsoList, ok := store.get(key)
				Expect(ok).To(BeFalse())
				Expect(httpsoList).To(BeNil())
			}
		})
	})
})
