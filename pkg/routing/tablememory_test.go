package routing

import (
	"fmt"
	"net/url"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	httpv1alpha1 "github.com/kedacore/http-add-on/operator/apis/http/v1alpha1"
	"github.com/kedacore/http-add-on/pkg/k8s"
)

var _ = Describe("TableMemory", func() {
	const (
		nameSuffix = "-br"
		hostSuffix = ".br"
	)

	var (
		httpso0 = httpv1alpha1.HTTPScaledObject{
			ObjectMeta: metav1.ObjectMeta{
				Name: "keda-sh",
			},
			Spec: httpv1alpha1.HTTPScaledObjectSpec{
				Hosts: []string{
					"keda.sh",
				},
			},
		}

		httpso0NamespacedName = *k8s.NamespacedNameFromObject(&httpso0)

		httpso1 = httpv1alpha1.HTTPScaledObject{
			ObjectMeta: metav1.ObjectMeta{
				Name: "one-one-one-one",
			},
			Spec: httpv1alpha1.HTTPScaledObjectSpec{
				Hosts: []string{
					"1.1.1.1",
				},
			},
		}

		httpso1NamespacedName = *k8s.NamespacedNameFromObject(&httpso1)

		httpsoList = httpv1alpha1.HTTPScaledObjectList{
			Items: []*httpv1alpha1.HTTPScaledObject{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "/",
					},
					Spec: httpv1alpha1.HTTPScaledObjectSpec{
						Hosts: []string{
							"localhost",
						},
						PathPrefixes: []string{
							"/",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "/f",
					},
					Spec: httpv1alpha1.HTTPScaledObjectSpec{
						Hosts: []string{
							"localhost",
						},
						PathPrefixes: []string{
							"/f",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "fo",
					},
					Spec: httpv1alpha1.HTTPScaledObjectSpec{
						Hosts: []string{
							"localhost",
						},
						PathPrefixes: []string{
							"fo",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "foo/",
					},
					Spec: httpv1alpha1.HTTPScaledObjectSpec{
						Hosts: []string{
							"localhost",
						},
						PathPrefixes: []string{
							"foo/",
						},
					},
				},
			},
		}

		assertIndex = func(tm tableMemory, input *httpv1alpha1.HTTPScaledObject, expected *httpv1alpha1.HTTPScaledObject) {
			okMatcher := BeTrue()
			if expected == nil {
				okMatcher = BeFalse()
			}

			httpsoMatcher := Equal(expected)
			if expected == nil {
				httpsoMatcher = BeNil()
			}

			namespacedName := k8s.NamespacedNameFromObject(input)
			indexKey := newTableMemoryIndexKey(namespacedName)
			httpso, ok := tm.index.get(indexKey)
			Expect(ok).To(okMatcher)
			Expect(httpso).To(httpsoMatcher)
		}

		assertStore = func(tm tableMemory, input *httpv1alpha1.HTTPScaledObject, expected *httpv1alpha1.HTTPScaledObject) {
			okMatcher := BeTrue()
			if expected == nil {
				okMatcher = BeFalse()
			}

			httpsoMatcher := ContainElements(expected)
			if expected == nil {
				httpsoMatcher = BeNil()
			}

			storeKeys := NewKeysFromHTTPSO(input)
			for _, storeKey := range storeKeys {
				httpSOList, ok := tm.store.get(storeKey)
				Expect(ok).To(okMatcher)
				if httpSOList == nil {
					Expect(httpSOList).To(httpsoMatcher)
				} else {
					Expect(httpSOList.Items).To(httpsoMatcher)
				}
			}
		}

		assertTrees = func(tm tableMemory, input *httpv1alpha1.HTTPScaledObject, expected *httpv1alpha1.HTTPScaledObject) {
			assertIndex(tm, input, expected)
			assertStore(tm, input, expected)
		}

		insertIndex = func(tm tableMemory, httpso *httpv1alpha1.HTTPScaledObject) tableMemory {
			namespacedName := k8s.NamespacedNameFromObject(httpso)
			indexKey := newTableMemoryIndexKey(namespacedName)
			tm.index, _, _ = tm.index.insert(indexKey, httpso)

			return tm
		}

		insertStore = func(tm tableMemory, httpso *httpv1alpha1.HTTPScaledObject) tableMemory {
			storeKeys := NewKeysFromHTTPSO(httpso)
			for _, storeKey := range storeKeys {
				tm.store, _, _ = tm.store.insert(storeKey, httpv1alpha1.NewHTTPScaledObjectList([]*httpv1alpha1.HTTPScaledObject{httpso}))
			}
			return tm
		}

		insertTrees = func(tm tableMemory, httpso *httpv1alpha1.HTTPScaledObject) tableMemory {
			tm = insertIndex(tm, httpso)
			tm = insertStore(tm, httpso)
			return tm
		}
	)

	Context("New", func() {
		It("returns a tableMemory with initialized tree", func() {
			i := NewTableMemory()

			tm, ok := i.(tableMemory)
			Expect(ok).To(BeTrue())
			Expect(tm.index).NotTo(BeNil())
			Expect(tm.store).NotTo(BeNil())
		})
	})

	Context("Remember", func() {
		It("returns a tableMemory with new object inserted", func() {
			tm := newTableMemory()
			tm = tm.Remember(&httpso0).(tableMemory)

			assertTrees(tm, &httpso0, &httpso0)
		})

		It("returns a tableMemory with new object inserted and other objects retained", func() {
			tm := newTableMemory()
			tm = tm.Remember(&httpso0).(tableMemory)
			tm = tm.Remember(&httpso1).(tableMemory)

			assertTrees(tm, &httpso1, &httpso1)
			assertTrees(tm, &httpso0, &httpso0)
		})

		It("returns a tableMemory with old object of same key replaced", func() {
			tm := newTableMemory()
			tm = tm.Remember(&httpso0).(tableMemory)

			httpso1 := *httpso0.DeepCopy()
			httpso1.Spec.TargetPendingRequests = ptr.To[int32](1)
			tm = tm.Remember(&httpso1).(tableMemory)

			assertTrees(tm, &httpso0, &httpso1)
		})

		It("returns a tableMemory with old object of same key replaced and other objects retained", func() {
			tm := newTableMemory()
			tm = tm.Remember(&httpso0).(tableMemory)
			tm = tm.Remember(&httpso1).(tableMemory)

			httpso2 := *httpso1.DeepCopy()
			httpso2.Spec.TargetPendingRequests = ptr.To[int32](1)
			tm = tm.Remember(&httpso2).(tableMemory)

			assertTrees(tm, &httpso1, &httpso2)
			assertTrees(tm, &httpso0, &httpso0)
		})

		It("returns a tableMemory with deep-copied object", func() {
			tm := newTableMemory()

			httpso := *httpso0.DeepCopy()
			tm = tm.Remember(&httpso).(tableMemory)

			httpso.Spec.Hosts[0] += hostSuffix
			assertTrees(tm, &httpso0, &httpso0)
		})

		It("gives precedence to the oldest object on conflict", func() {
			tm := newTableMemory()

			t0 := time.Now()

			httpso00 := *httpso0.DeepCopy()
			httpso00.ObjectMeta.CreationTimestamp = metav1.NewTime(t0)
			tm = tm.Remember(&httpso00).(tableMemory)

			httpso01 := *httpso0.DeepCopy()
			httpso01.ObjectMeta.Name += nameSuffix
			httpso01.ObjectMeta.CreationTimestamp = metav1.NewTime(t0.Add(-time.Minute))
			tm = tm.Remember(&httpso01).(tableMemory)

			httpso10 := *httpso1.DeepCopy()
			httpso10.ObjectMeta.CreationTimestamp = metav1.NewTime(t0)
			tm = tm.Remember(&httpso10).(tableMemory)

			httpso11 := *httpso1.DeepCopy()
			httpso11.ObjectMeta.Name += nameSuffix
			httpso11.ObjectMeta.CreationTimestamp = metav1.NewTime(t0.Add(+time.Minute))
			tm = tm.Remember(&httpso11).(tableMemory)

			assertIndex(tm, &httpso00, &httpso00)
			assertStore(tm, &httpso00, &httpso01)

			assertIndex(tm, &httpso01, &httpso01)
			assertStore(tm, &httpso01, &httpso01)

			assertIndex(tm, &httpso10, &httpso10)
			assertStore(tm, &httpso10, &httpso10)

			assertIndex(tm, &httpso11, &httpso11)
			assertStore(tm, &httpso11, &httpso10)
		})
	})

	Context("Forget", func() {
		It("returns a tableMemory with old object deleted", func() {
			tm := newTableMemory()
			tm = insertTrees(tm, &httpso0)

			tm = tm.Forget(&httpso0NamespacedName).(tableMemory)

			assertTrees(tm, &httpso0, nil)
		})

		It("returns a tableMemory with old object deleted and other objects retained", func() {
			tm := newTableMemory()
			tm = insertTrees(tm, &httpso0)
			tm = insertTrees(tm, &httpso1)

			tm = tm.Forget(&httpso0NamespacedName).(tableMemory)

			assertTrees(tm, &httpso1, &httpso1)
			assertTrees(tm, &httpso0, nil)
		})

		It("returns unchanged tableMemory when object is absent", func() {
			tm := newTableMemory()
			tm = insertTrees(tm, &httpso0)

			index0 := *tm.index
			store0 := *tm.store
			tm = tm.Forget(&httpso1NamespacedName).(tableMemory)
			index1 := *tm.index
			store1 := *tm.store
			Expect(index1).To(Equal(index0))
			Expect(store1).To(Equal(store0))
		})

		It("forgets only when namespaced names match on conflict", func() {
			tm := newTableMemory()
			tm = insertTrees(tm, &httpso0)

			t0 := time.Now()

			httpso00 := *httpso0.DeepCopy()
			httpso00.ObjectMeta.CreationTimestamp = metav1.NewTime(t0)
			tm = insertTrees(tm, &httpso00)

			httpso01 := *httpso0.DeepCopy()
			httpso01.ObjectMeta.Name += nameSuffix
			httpso01.ObjectMeta.CreationTimestamp = metav1.NewTime(t0.Add(-time.Minute))
			tm = insertTrees(tm, &httpso01)

			httpso10 := *httpso1.DeepCopy()
			httpso10.ObjectMeta.Name += nameSuffix
			httpso10.ObjectMeta.CreationTimestamp = metav1.NewTime(t0)
			tm = insertTrees(tm, &httpso10)

			httpso11 := *httpso1.DeepCopy()
			httpso11.ObjectMeta.CreationTimestamp = metav1.NewTime(t0.Add(-time.Minute))
			tm = insertTrees(tm, &httpso11)

			tm = tm.Forget(&httpso0NamespacedName).(tableMemory)
			tm = tm.Forget(&httpso1NamespacedName).(tableMemory)

			assertIndex(tm, &httpso00, nil)
			assertStore(tm, &httpso00, &httpso01)

			assertIndex(tm, &httpso01, &httpso01)
			assertStore(tm, &httpso01, &httpso01)

			assertIndex(tm, &httpso10, &httpso10)
			assertStore(tm, &httpso10, nil)

			assertIndex(tm, &httpso11, nil)
			assertStore(tm, &httpso11, nil)
		})
	})

	Context("Recall", func() {
		It("returns object with matching key", func() {
			tm := newTableMemory()
			tm = insertTrees(tm, &httpso0)

			httpso := tm.Recall(&httpso0NamespacedName)
			Expect(httpso).To(Equal(&httpso0))
		})

		It("returns nil when object is absent", func() {
			tm := newTableMemory()
			tm = insertTrees(tm, &httpso0)

			httpso := tm.Recall(&httpso1NamespacedName)
			Expect(httpso).To(BeNil())
		})

		It("returns deep-copied object", func() {
			tm := newTableMemory()
			tm = insertTrees(tm, &httpso0)

			httpso := tm.Recall(&httpso0NamespacedName)
			Expect(httpso).To(Equal(&httpso0))

			httpso.Spec.Hosts[0] += hostSuffix

			assertTrees(tm, &httpso0, &httpso0)
		})
	})

	Context("Route", func() {
		It("returns nil when no matching host for URL", func() {
			tm := newTableMemory()
			tm = insertTrees(tm, &httpso0)

			url, err := url.Parse(fmt.Sprintf("https://%s.br", httpso0.Spec.Hosts[0]))
			Expect(err).NotTo(HaveOccurred())
			Expect(url).NotTo(BeNil())
			urlKey := NewKeyFromURL(url)
			Expect(urlKey).NotTo(BeNil())
			httpso := tm.Route(urlKey)
			Expect(httpso).To(BeNil())
		})

		It("returns expected object with matching host for URL", func() {
			tm := newTableMemory()
			tm = insertTrees(tm, &httpso0)
			tm = insertTrees(tm, &httpso1)

			//goland:noinspection HttpUrlsUsage
			url0, err := url.Parse(fmt.Sprintf("http://%s", httpso0.Spec.Hosts[0]))
			Expect(err).NotTo(HaveOccurred())
			Expect(url0).NotTo(BeNil())
			url0Key := NewKeyFromURL(url0)
			Expect(url0Key).NotTo(BeNil())
			ret0 := tm.Route(url0Key)
			Expect(ret0).To(Equal(&httpso0))

			url1, err := url.Parse(fmt.Sprintf("https://%s:443/abc/def?123=456#789", httpso1.Spec.Hosts[0]))
			Expect(err).NotTo(HaveOccurred())
			Expect(url1).NotTo(BeNil())
			url1Key := NewKeyFromURL(url1)
			Expect(url1Key).NotTo(BeNil())
			ret1 := tm.Route(url1Key)
			Expect(ret1).To(Equal(&httpso1))
		})

		It("returns nil when no matching pathPrefix for URL", func() {
			var (
				httpsoFoo = httpsoList.Items[3]
			)

			tm := newTableMemory()
			tm = insertTrees(tm, httpsoFoo)

			//goland:noinspection HttpUrlsUsage
			url, err := url.Parse(fmt.Sprintf("http://%s/bar%s", httpsoFoo.Spec.Hosts[0], httpsoFoo.Spec.PathPrefixes[0]))
			Expect(err).NotTo(HaveOccurred())
			Expect(url).NotTo(BeNil())
			urlKey := NewKeyFromURL(url)
			Expect(urlKey).NotTo(BeNil())
			httpso := tm.Route(urlKey)
			Expect(httpso).To(BeNil())
		})

		It("returns expected object with matching pathPrefix for URL", func() {
			tm := newTableMemory()
			for _, httpso := range httpsoList.Items {
				httpso := httpso

				tm = insertTrees(tm, httpso)
			}

			for _, httpso := range httpsoList.Items {
				url, err := url.Parse(fmt.Sprintf("https://%s/%s", httpso.Spec.Hosts[0], httpso.Spec.PathPrefixes[0]))
				Expect(err).NotTo(HaveOccurred())
				Expect(url).NotTo(BeNil())
				urlKey := NewKeyFromURL(url)
				Expect(urlKey).NotTo(BeNil())
				ret := tm.Route(urlKey)
				Expect(ret).To(Equal(httpso))
			}

			for _, httpso := range httpsoList.Items {
				url, err := url.Parse(fmt.Sprintf("https://%s/%s/bar", httpso.Spec.Hosts[0], httpso.Spec.PathPrefixes[0]))
				Expect(err).NotTo(HaveOccurred())
				Expect(url).NotTo(BeNil())
				urlKey := NewKeyFromURL(url)
				Expect(urlKey).NotTo(BeNil())
				ret := tm.Route(urlKey)
				Expect(ret).To(Equal(httpso))
			}
		})
	})

	Context("E2E", func() {
		It("succeeds", func() {
			tm := NewTableMemory()

			ret0 := tm.Recall(&httpso0NamespacedName)
			Expect(ret0).To(BeNil())

			tm = tm.Remember(&httpso0)

			ret1 := tm.Recall(&httpso0NamespacedName)
			Expect(ret1).To(Equal(&httpso0))

			tm = tm.Forget(&httpso0NamespacedName)

			ret2 := tm.Recall(&httpso0NamespacedName)
			Expect(ret2).To(BeNil())

			tm = tm.Remember(&httpso0)
			tm = tm.Remember(&httpso1)

			ret3 := tm.Recall(&httpso0NamespacedName)
			Expect(ret3).To(Equal(&httpso0))

			ret4 := tm.Recall(&httpso1NamespacedName)
			Expect(ret4).To(Equal(&httpso1))

			//goland:noinspection HttpUrlsUsage
			url0, err := url.Parse(fmt.Sprintf("http://%s:80?123=456#789", httpso0.Spec.Hosts[0]))
			Expect(err).NotTo(HaveOccurred())
			Expect(url0).NotTo(BeNil())

			url0Key := NewKeyFromURL(url0)
			Expect(url0Key).NotTo(BeNil())

			ret5 := tm.Route(url0Key)
			Expect(ret5).To(Equal(&httpso0))

			url1, err := url.Parse(fmt.Sprintf("https://user:pass@%s:443/abc/def", httpso1.Spec.Hosts[0]))
			Expect(err).NotTo(HaveOccurred())
			Expect(url1).NotTo(BeNil())

			url1Key := NewKeyFromURL(url1)
			Expect(url1Key).NotTo(BeNil())

			ret6 := tm.Route(url1Key)
			Expect(ret6).To(Equal(&httpso1))

			url2, err := url.Parse("http://0.0.0.0")
			Expect(err).NotTo(HaveOccurred())
			Expect(url2).NotTo(BeNil())

			url2Key := NewKeyFromURL(url2)
			Expect(url2Key).NotTo(BeNil())

			ret7 := tm.Route(url2Key)
			Expect(ret7).To(BeNil())

			tm = tm.Forget(&httpso0NamespacedName)

			ret8 := tm.Route(url0Key)
			Expect(ret8).To(BeNil())

			httpso := *httpso1.DeepCopy()
			httpso.Spec.TargetPendingRequests = ptr.To[int32](1)

			tm = tm.Remember(&httpso)

			ret9 := tm.Route(url1Key)
			Expect(ret9).To(Equal(&httpso1))
		})
	})
})
