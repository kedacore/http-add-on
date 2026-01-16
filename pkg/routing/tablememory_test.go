package routing

import (
	"testing"
	"time"

	iradix "github.com/hashicorp/go-immutable-radix/v2"
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
			Items: []httpv1alpha1.HTTPScaledObject{
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
			httpso, ok := tm.index.Get(indexKey)
			Expect(ok).To(okMatcher)
			Expect(httpso).To(httpsoMatcher)
		}

		assertStore = func(tm tableMemory, input *httpv1alpha1.HTTPScaledObject, expected *httpv1alpha1.HTTPScaledObject) {
			okMatcher := BeTrue()
			if expected == nil {
				okMatcher = BeFalse()
			}

			httpsoMatcher := Equal(expected)
			if expected == nil {
				httpsoMatcher = BeNil()
			}

			storeKeys := NewKeysFromHTTPSO(input)
			for _, storeKey := range storeKeys {
				httpso, ok := tm.store.Get(storeKey)
				Expect(ok).To(okMatcher)
				Expect(httpso).To(httpsoMatcher)
			}
		}

		assertTrees = func(tm tableMemory, input *httpv1alpha1.HTTPScaledObject, expected *httpv1alpha1.HTTPScaledObject) {
			assertIndex(tm, input, expected)
			assertStore(tm, input, expected)
		}

		insertIndex = func(tm tableMemory, httpso *httpv1alpha1.HTTPScaledObject) tableMemory {
			namespacedName := k8s.NamespacedNameFromObject(httpso)
			indexKey := newTableMemoryIndexKey(namespacedName)
			tm.index, _, _ = tm.index.Insert(indexKey, httpso)

			return tm
		}

		insertStore = func(tm tableMemory, httpso *httpv1alpha1.HTTPScaledObject) tableMemory {
			storeKeys := NewKeysFromHTTPSO(httpso)
			for _, storeKey := range storeKeys {
				tm.store, _, _ = tm.store.Insert(storeKey, httpso)
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
			tm := tableMemory{
				index: iradix.New[*httpv1alpha1.HTTPScaledObject](),
				store: iradix.New[*httpv1alpha1.HTTPScaledObject](),
			}
			tm = tm.Remember(&httpso0).(tableMemory)

			assertTrees(tm, &httpso0, &httpso0)
		})

		It("returns a tableMemory with new object inserted and other objects retained", func() {
			tm := tableMemory{
				index: iradix.New[*httpv1alpha1.HTTPScaledObject](),
				store: iradix.New[*httpv1alpha1.HTTPScaledObject](),
			}
			tm = tm.Remember(&httpso0).(tableMemory)
			tm = tm.Remember(&httpso1).(tableMemory)

			assertTrees(tm, &httpso1, &httpso1)
			assertTrees(tm, &httpso0, &httpso0)
		})

		It("returns a tableMemory with old object of same key replaced", func() {
			tm := tableMemory{
				index: iradix.New[*httpv1alpha1.HTTPScaledObject](),
				store: iradix.New[*httpv1alpha1.HTTPScaledObject](),
			}
			tm = tm.Remember(&httpso0).(tableMemory)

			httpso1 := *httpso0.DeepCopy()
			httpso1.Spec.TargetPendingRequests = ptr.To[int32](1)
			tm = tm.Remember(&httpso1).(tableMemory)

			assertTrees(tm, &httpso0, &httpso1)
		})

		It("returns a tableMemory with old object of same key replaced and other objects retained", func() {
			tm := tableMemory{
				index: iradix.New[*httpv1alpha1.HTTPScaledObject](),
				store: iradix.New[*httpv1alpha1.HTTPScaledObject](),
			}
			tm = tm.Remember(&httpso0).(tableMemory)
			tm = tm.Remember(&httpso1).(tableMemory)

			httpso2 := *httpso1.DeepCopy()
			httpso2.Spec.TargetPendingRequests = ptr.To[int32](1)
			tm = tm.Remember(&httpso2).(tableMemory)

			assertTrees(tm, &httpso1, &httpso2)
			assertTrees(tm, &httpso0, &httpso0)
		})

		It("returns a tableMemory with deep-copied object", func() {
			tm := tableMemory{
				index: iradix.New[*httpv1alpha1.HTTPScaledObject](),
				store: iradix.New[*httpv1alpha1.HTTPScaledObject](),
			}

			httpso := *httpso0.DeepCopy()
			tm = tm.Remember(&httpso).(tableMemory)

			httpso.Spec.Hosts[0] += hostSuffix
			assertTrees(tm, &httpso0, &httpso0)
		})

		It("gives precedence to the oldest object on conflict", func() {
			tm := tableMemory{
				index: iradix.New[*httpv1alpha1.HTTPScaledObject](),
				store: iradix.New[*httpv1alpha1.HTTPScaledObject](),
			}

			t0 := time.Now()

			httpso00 := *httpso0.DeepCopy()
			httpso00.CreationTimestamp = metav1.NewTime(t0)
			tm = tm.Remember(&httpso00).(tableMemory)

			httpso01 := *httpso0.DeepCopy()
			httpso01.Name += nameSuffix
			httpso01.CreationTimestamp = metav1.NewTime(t0.Add(-time.Minute))
			tm = tm.Remember(&httpso01).(tableMemory)

			httpso10 := *httpso1.DeepCopy()
			httpso10.CreationTimestamp = metav1.NewTime(t0)
			tm = tm.Remember(&httpso10).(tableMemory)

			httpso11 := *httpso1.DeepCopy()
			httpso11.Name += nameSuffix
			httpso11.CreationTimestamp = metav1.NewTime(t0.Add(+time.Minute))
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
			tm := tableMemory{
				index: iradix.New[*httpv1alpha1.HTTPScaledObject](),
				store: iradix.New[*httpv1alpha1.HTTPScaledObject](),
			}
			tm = insertTrees(tm, &httpso0)

			tm = tm.Forget(&httpso0NamespacedName).(tableMemory)

			assertTrees(tm, &httpso0, nil)
		})

		It("returns a tableMemory with old object deleted and other objects retained", func() {
			tm := tableMemory{
				index: iradix.New[*httpv1alpha1.HTTPScaledObject](),
				store: iradix.New[*httpv1alpha1.HTTPScaledObject](),
			}
			tm = insertTrees(tm, &httpso0)
			tm = insertTrees(tm, &httpso1)

			tm = tm.Forget(&httpso0NamespacedName).(tableMemory)

			assertTrees(tm, &httpso1, &httpso1)
			assertTrees(tm, &httpso0, nil)
		})

		It("returns unchanged tableMemory when object is absent", func() {
			tm := tableMemory{
				index: iradix.New[*httpv1alpha1.HTTPScaledObject](),
				store: iradix.New[*httpv1alpha1.HTTPScaledObject](),
			}
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
			tm := tableMemory{
				index: iradix.New[*httpv1alpha1.HTTPScaledObject](),
				store: iradix.New[*httpv1alpha1.HTTPScaledObject](),
			}
			tm = insertTrees(tm, &httpso0)

			t0 := time.Now()

			httpso00 := *httpso0.DeepCopy()
			httpso00.CreationTimestamp = metav1.NewTime(t0)
			tm = insertTrees(tm, &httpso00)

			httpso01 := *httpso0.DeepCopy()
			httpso01.Name += nameSuffix
			httpso01.CreationTimestamp = metav1.NewTime(t0.Add(-time.Minute))
			tm = insertTrees(tm, &httpso01)

			httpso10 := *httpso1.DeepCopy()
			httpso10.Name += nameSuffix
			httpso10.CreationTimestamp = metav1.NewTime(t0)
			tm = insertTrees(tm, &httpso10)

			httpso11 := *httpso1.DeepCopy()
			httpso11.CreationTimestamp = metav1.NewTime(t0.Add(-time.Minute))
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
			tm := tableMemory{
				index: iradix.New[*httpv1alpha1.HTTPScaledObject](),
				store: iradix.New[*httpv1alpha1.HTTPScaledObject](),
			}
			tm = insertTrees(tm, &httpso0)

			httpso := tm.Recall(&httpso0NamespacedName)
			Expect(httpso).To(Equal(&httpso0))
		})

		It("returns nil when object is absent", func() {
			tm := tableMemory{
				index: iradix.New[*httpv1alpha1.HTTPScaledObject](),
				store: iradix.New[*httpv1alpha1.HTTPScaledObject](),
			}
			tm = insertTrees(tm, &httpso0)

			httpso := tm.Recall(&httpso1NamespacedName)
			Expect(httpso).To(BeNil())
		})

		It("returns deep-copied object", func() {
			tm := tableMemory{
				index: iradix.New[*httpv1alpha1.HTTPScaledObject](),
				store: iradix.New[*httpv1alpha1.HTTPScaledObject](),
			}
			tm = insertTrees(tm, &httpso0)

			httpso := tm.Recall(&httpso0NamespacedName)
			Expect(httpso).To(Equal(&httpso0))

			httpso.Spec.Hosts[0] += hostSuffix

			assertTrees(tm, &httpso0, &httpso0)
		})
	})

	Context("Route", func() {
		It("returns nil when no matching host", func() {
			tm := tableMemory{
				index: iradix.New[*httpv1alpha1.HTTPScaledObject](),
				store: iradix.New[*httpv1alpha1.HTTPScaledObject](),
			}
			tm = insertTrees(tm, &httpso0)

			httpso := tm.Route(httpso0.Spec.Hosts[0]+".br", "")
			Expect(httpso).To(BeNil())
		})

		It("returns expected object with matching host", func() {
			tm := tableMemory{
				index: iradix.New[*httpv1alpha1.HTTPScaledObject](),
				store: iradix.New[*httpv1alpha1.HTTPScaledObject](),
			}
			tm = insertTrees(tm, &httpso0)
			tm = insertTrees(tm, &httpso1)

			ret0 := tm.Route(httpso0.Spec.Hosts[0], "")
			Expect(ret0).To(Equal(&httpso0))

			ret1 := tm.Route(httpso1.Spec.Hosts[0], "/abc/def")
			Expect(ret1).To(Equal(&httpso1))
		})

		It("returns nil when no matching pathPrefix", func() {
			httpsoFoo := httpsoList.Items[3]

			tm := tableMemory{
				index: iradix.New[*httpv1alpha1.HTTPScaledObject](),
				store: iradix.New[*httpv1alpha1.HTTPScaledObject](),
			}
			tm = insertTrees(tm, &httpsoFoo)

			httpso := tm.Route(httpsoFoo.Spec.Hosts[0], "/bar"+httpsoFoo.Spec.PathPrefixes[0])
			Expect(httpso).To(BeNil())
		})

		It("returns expected object with matching pathPrefix", func() {
			tm := tableMemory{
				index: iradix.New[*httpv1alpha1.HTTPScaledObject](),
				store: iradix.New[*httpv1alpha1.HTTPScaledObject](),
			}
			for _, httpso := range httpsoList.Items {
				tm = insertTrees(tm, &httpso)
			}

			for _, httpso := range httpsoList.Items {
				ret := tm.Route(httpso.Spec.Hosts[0], httpso.Spec.PathPrefixes[0])
				Expect(ret).To(Equal(&httpso))
			}

			for _, httpso := range httpsoList.Items {
				ret := tm.Route(httpso.Spec.Hosts[0], httpso.Spec.PathPrefixes[0]+"/bar")
				Expect(ret).To(Equal(&httpso))
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

			ret5 := tm.Route(httpso0.Spec.Hosts[0], "")
			Expect(ret5).To(Equal(&httpso0))

			ret6 := tm.Route(httpso1.Spec.Hosts[0], "/abc/def")
			Expect(ret6).To(Equal(&httpso1))

			ret7 := tm.Route("0.0.0.0", "")
			Expect(ret7).To(BeNil())

			tm = tm.Forget(&httpso0NamespacedName)

			ret8 := tm.Route(httpso0.Spec.Hosts[0], "")
			Expect(ret8).To(BeNil())

			httpso := *httpso1.DeepCopy()
			httpso.Spec.TargetPendingRequests = ptr.To[int32](1)

			tm = tm.Remember(&httpso)

			ret9 := tm.Route(httpso1.Spec.Hosts[0], "/abc/def")
			Expect(ret9).To(Equal(&httpso))
		})
	})
})

func TestRouteWildcardMultiLevel(t *testing.T) {
	wildcardHTTPSO := &httpv1alpha1.HTTPScaledObject{
		ObjectMeta: metav1.ObjectMeta{Name: "wildcard-example"},
		Spec:       httpv1alpha1.HTTPScaledObjectSpec{Hosts: []string{"*.example.com"}},
	}

	t.Run("matches single-level subdomain", func(t *testing.T) {
		runRouteTest(t, []*httpv1alpha1.HTTPScaledObject{wildcardHTTPSO},
			"bar.example.com", "", wildcardHTTPSO)
	})

	t.Run("matches nested subdomain", func(t *testing.T) {
		runRouteTest(t, []*httpv1alpha1.HTTPScaledObject{wildcardHTTPSO},
			"foo.bar.example.com", "", wildcardHTTPSO)
	})

	t.Run("matches deeply nested subdomain", func(t *testing.T) {
		runRouteTest(t, []*httpv1alpha1.HTTPScaledObject{wildcardHTTPSO},
			"a.b.c.example.com", "", wildcardHTTPSO)
	})

	t.Run("rejects different domain", func(t *testing.T) {
		runRouteTest(t, []*httpv1alpha1.HTTPScaledObject{wildcardHTTPSO},
			"foo.other.com", "", nil)
	})
}

func TestRouteWildcardPrecedence(t *testing.T) {
	wildcardHTTPSO := &httpv1alpha1.HTTPScaledObject{
		ObjectMeta: metav1.ObjectMeta{Name: "wildcard-example"},
		Spec:       httpv1alpha1.HTTPScaledObjectSpec{Hosts: []string{"*.example.com"}},
	}
	moreSpecificWildcardHTTPSO := &httpv1alpha1.HTTPScaledObject{
		ObjectMeta: metav1.ObjectMeta{Name: "wildcard-bar-example"},
		Spec:       httpv1alpha1.HTTPScaledObjectSpec{Hosts: []string{"*.bar.example.com"}},
	}
	exactHTTPSO := &httpv1alpha1.HTTPScaledObject{
		ObjectMeta: metav1.ObjectMeta{Name: "exact-foo"},
		Spec:       httpv1alpha1.HTTPScaledObjectSpec{Hosts: []string{"foo.example.com"}},
	}
	catchAllHTTPSO := &httpv1alpha1.HTTPScaledObject{
		ObjectMeta: metav1.ObjectMeta{Name: "catch-all"},
		Spec:       httpv1alpha1.HTTPScaledObjectSpec{Hosts: []string{"*"}},
	}

	t.Run("exact wins over wildcard", func(t *testing.T) {
		runRouteTest(t, []*httpv1alpha1.HTTPScaledObject{wildcardHTTPSO, exactHTTPSO},
			"foo.example.com", "", exactHTTPSO)
	})

	t.Run("exact wins regardless of storage order", func(t *testing.T) {
		runRouteTest(t, []*httpv1alpha1.HTTPScaledObject{exactHTTPSO, wildcardHTTPSO},
			"foo.example.com", "", exactHTTPSO)
	})

	t.Run("more specific wildcard wins", func(t *testing.T) {
		runRouteTest(t, []*httpv1alpha1.HTTPScaledObject{wildcardHTTPSO, moreSpecificWildcardHTTPSO},
			"foo.bar.example.com", "", moreSpecificWildcardHTTPSO)
	})

	t.Run("falls back to less specific wildcard", func(t *testing.T) {
		runRouteTest(t, []*httpv1alpha1.HTTPScaledObject{wildcardHTTPSO, moreSpecificWildcardHTTPSO},
			"foo.baz.example.com", "", wildcardHTTPSO)
	})

	t.Run("wildcard wins over catch-all", func(t *testing.T) {
		runRouteTest(t, []*httpv1alpha1.HTTPScaledObject{catchAllHTTPSO, wildcardHTTPSO},
			"bar.example.com", "", wildcardHTTPSO)
	})

	t.Run("falls back to catch-all when no wildcard matches", func(t *testing.T) {
		runRouteTest(t, []*httpv1alpha1.HTTPScaledObject{catchAllHTTPSO, wildcardHTTPSO},
			"bar.other.com", "", catchAllHTTPSO)
	})
}

func TestRouteCatchAll(t *testing.T) {
	catchAllHTTPSO := &httpv1alpha1.HTTPScaledObject{
		ObjectMeta: metav1.ObjectMeta{Name: "catch-all"},
		Spec:       httpv1alpha1.HTTPScaledObjectSpec{Hosts: []string{"*"}},
	}
	emptyHostHTTPSO := &httpv1alpha1.HTTPScaledObject{
		ObjectMeta: metav1.ObjectMeta{Name: "empty-host"},
		Spec:       httpv1alpha1.HTTPScaledObjectSpec{Hosts: []string{""}},
	}
	nilHostHTTPSO := &httpv1alpha1.HTTPScaledObject{
		ObjectMeta: metav1.ObjectMeta{Name: "nil-host"},
		Spec:       httpv1alpha1.HTTPScaledObjectSpec{Hosts: nil},
	}

	t.Run("star matches single-label host", func(t *testing.T) {
		runRouteTest(t, []*httpv1alpha1.HTTPScaledObject{catchAllHTTPSO},
			"localhost", "", catchAllHTTPSO)
	})

	t.Run("star matches multi-label host", func(t *testing.T) {
		runRouteTest(t, []*httpv1alpha1.HTTPScaledObject{catchAllHTTPSO},
			"foo.example.com", "", catchAllHTTPSO)
	})

	t.Run("empty host matches any hostname", func(t *testing.T) {
		runRouteTest(t, []*httpv1alpha1.HTTPScaledObject{emptyHostHTTPSO},
			"anything.example.com", "", emptyHostHTTPSO)
	})

	t.Run("nil hosts matches any hostname", func(t *testing.T) {
		runRouteTest(t, []*httpv1alpha1.HTTPScaledObject{nilHostHTTPSO},
			"example.com", "", nilHostHTTPSO)
	})
}

func TestRouteWildcardWithPath(t *testing.T) {
	wildcardWithPathHTTPSO := &httpv1alpha1.HTTPScaledObject{
		ObjectMeta: metav1.ObjectMeta{Name: "wildcard-with-path"},
		Spec: httpv1alpha1.HTTPScaledObjectSpec{
			Hosts:        []string{"*.example.com"},
			PathPrefixes: []string{"/api/"},
		},
	}

	t.Run("matches with path prefix", func(t *testing.T) {
		runRouteTest(t, []*httpv1alpha1.HTTPScaledObject{wildcardWithPathHTTPSO},
			"bar.example.com", "/api/v1/users", wildcardWithPathHTTPSO)
	})

	t.Run("rejects wrong path", func(t *testing.T) {
		runRouteTest(t, []*httpv1alpha1.HTTPScaledObject{wildcardWithPathHTTPSO},
			"bar.example.com", "/other/", nil)
	})
}

// runRouteTest is a helper that creates a TableMemory, stores the given
// HTTPScaledObjects, and verifies that Route returns the expected result.
func runRouteTest(t *testing.T, stored []*httpv1alpha1.HTTPScaledObject, reqHost, reqPath string, want *httpv1alpha1.HTTPScaledObject) {
	t.Helper()
	tm := NewTableMemory()
	for _, httpso := range stored {
		tm = tm.Remember(httpso)
	}

	route := tm.Route(reqHost, reqPath)

	switch {
	case route == nil && want == nil:
		// ok
	case route == nil:
		t.Errorf("route=nil, want=%q", want.Name)
	case want == nil:
		t.Errorf("route=%q, want=nil", route.Name)
	case route.Name != want.Name:
		t.Errorf("route=%q, want=%q", route.Name, want.Name)
	}
}
