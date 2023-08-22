package routing

import (
	"context"
	"time"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/utils/ptr"

	httpv1alpha1 "github.com/kedacore/http-add-on/operator/apis/http/v1alpha1"
	clientsetmock "github.com/kedacore/http-add-on/operator/generated/clientset/versioned/mock"
	clientsethttpv1alpha1mock "github.com/kedacore/http-add-on/operator/generated/clientset/versioned/typed/http/v1alpha1/mock"
	informersexternalversions "github.com/kedacore/http-add-on/operator/generated/informers/externalversions"
	"github.com/kedacore/http-add-on/pkg/k8s"
	"github.com/kedacore/http-add-on/pkg/util"
)

var _ = Describe("Table", func() {
	const (
		namespace = "default"
	)

	var (
		ctrl                  *gomock.Controller
		watcher               *watch.FakeWatcher
		httpsoCl              *clientsethttpv1alpha1mock.MockHTTPScaledObjectInterface
		sharedInformerFactory informersexternalversions.SharedInformerFactory

		ctx        = context.Background()
		httpsoList = httpv1alpha1.HTTPScaledObjectList{
			Items: []httpv1alpha1.HTTPScaledObject{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: namespace,
						Name:      "keda-sh",
					},
					Spec: httpv1alpha1.HTTPScaledObjectSpec{
						Hosts: []string{
							"keda.sh",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: namespace,
						Name:      "kubernetes-io",
					},
					Spec: httpv1alpha1.HTTPScaledObjectSpec{
						Hosts: []string{
							"kubernetes.io",
						},
						TargetPendingRequests: ptr.To[int32](1),
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: namespace,
						Name:      "github-com",
					},
					Spec: httpv1alpha1.HTTPScaledObjectSpec{
						Hosts: []string{
							"github.com",
						},
						Replicas: &httpv1alpha1.ReplicaStruct{
							Min: ptr.To[int32](3),
						},
					},
				},
			},
		}
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		watcher = watch.NewFake()

		httpsoCl = clientsethttpv1alpha1mock.NewMockHTTPScaledObjectInterface(ctrl)
		httpsoCl.EXPECT().
			List(gomock.Any(), gomock.Any()).
			Return(&httpsoList, nil).
			AnyTimes()
		httpsoCl.EXPECT().
			Watch(gomock.Any(), gomock.Any()).
			Return(watcher, nil).
			AnyTimes()

		httpv1alpha1 := clientsethttpv1alpha1mock.NewMockHttpV1alpha1Interface(ctrl)
		httpv1alpha1.EXPECT().
			HTTPScaledObjects(namespace).
			Return(httpsoCl).
			AnyTimes()

		clientset := clientsetmock.NewMockInterface(ctrl)
		clientset.EXPECT().
			HttpV1alpha1().
			Return(httpv1alpha1).
			AnyTimes()

		sharedInformerFactory = informersexternalversions.NewSharedInformerFactory(clientset, 0)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Context("New", func() {
		It("returns a table with fields initialized", func() {
			i, err := NewTable(sharedInformerFactory, namespace)
			Expect(err).NotTo(HaveOccurred())
			Expect(i).NotTo(BeNil())

			t, ok := i.(*table)
			Expect(ok).To(BeTrue())
			Expect(t.httpScaledObjectInformer).NotTo(BeNil())
			Expect(t.httpScaledObjectEventHandlerRegistration).NotTo(BeNil())
			Expect(t.httpScaledObjects).NotTo(BeNil())
			Expect(t.memorySignaler).NotTo(BeNil())

			// TODO(pedrotorres): mock to check registration
			// TODO(pedrotorres): refactor to check namespace
		})

		// TODO(pedrotorres): test code path where informer is not sharedIndexInformer
		// TODO(pedrotorres): test code path where informer#AddEventHandler fails
	})

	Context("runInformer", func() {
		var (
			t *table
		)

		BeforeEach(func() {
			i, _ := NewTable(sharedInformerFactory, namespace)
			t = i.(*table)
		})

		It("starts shared informer factory", func() {
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()

			go util.IgnoringError(util.ApplyContext(t.runInformer, ctx))

			time.Sleep(time.Second)

			b := t.httpScaledObjectInformer.HasStarted()
			Expect(b).To(BeTrue())
		})

		It("returns when context is done", func() {
			ctx, cancel := context.WithCancel(ctx)
			cancel()

			err := util.WithTimeout(time.Second, util.ApplyContext(t.runInformer, ctx))
			Expect(err).To(MatchError(context.Canceled))
		})

		It("returns when informer has already started", func() {
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()

			go t.httpScaledObjectInformer.Run(ctx.Done())

			time.Sleep(time.Second)

			err := util.WithTimeout(time.Second, util.ApplyContext(t.runInformer, ctx))
			Expect(err).To(MatchError(errStartedSharedIndexInformer))
		})

		// TODO(pedrotorres): test code path where informer stops
	})

	Context("refreshMemory", func() {
		var (
			t *table
		)

		BeforeEach(func() {
			i, _ := NewTable(sharedInformerFactory, namespace)
			t = i.(*table)
		})

		It("refreshes memory on first iteration", func() {
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()

			for _, httpso := range httpsoList.Items {
				httpso := httpso

				key := *k8s.NamespacedNameFromObject(&httpso)
				t.httpScaledObjects[key] = &httpso
			}

			go util.IgnoringError(util.ApplyContext(t.runInformer, ctx))
			go util.IgnoringError(util.ApplyContext(t.refreshMemory, ctx))

			time.Sleep(2 * time.Second)

			tm := t.memoryHolder.Get()
			Expect(tm).NotTo(BeNil())

			for _, httpso := range httpsoList.Items {
				namespacedName := k8s.NamespacedNameFromObject(&httpso)
				ret := tm.Recall(namespacedName)
				Expect(ret).To(Equal(&httpso))
			}
		})

		It("refreshes memory after signal", func() {
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()

			for _, httpso := range httpsoList.Items {
				httpso := httpso

				key := *k8s.NamespacedNameFromObject(&httpso)
				t.httpScaledObjects[key] = &httpso
			}

			go util.IgnoringError(util.ApplyContext(t.runInformer, ctx))
			go util.IgnoringError(util.ApplyContext(t.refreshMemory, ctx))

			time.Sleep(2 * time.Second)

			httpso := httpv1alpha1.HTTPScaledObject{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespace,
					Name:      "azure-com",
				},
				Spec: httpv1alpha1.HTTPScaledObjectSpec{
					Hosts: []string{
						"azure.com",
					},
					Replicas: &httpv1alpha1.ReplicaStruct{
						Min: ptr.To[int32](3),
					},
				},
			}
			t.httpScaledObjects[*k8s.NamespacedNameFromObject(&httpso)] = &httpso

			first := httpsoList.Items[0]
			delete(t.httpScaledObjects, *k8s.NamespacedNameFromObject(&first))

			t.memorySignaler.Signal()

			time.Sleep(time.Second)

			tm := t.memoryHolder.Get()
			Expect(tm).NotTo(BeNil())

			for _, httpso := range append(httpsoList.Items[1:], httpso) {
				namespacedName := k8s.NamespacedNameFromObject(&httpso)
				ret := tm.Recall(namespacedName)
				Expect(ret).To(Equal(&httpso))
			}

			namespacedName := k8s.NamespacedNameFromObject(&first)
			ret := tm.Recall(namespacedName)
			Expect(ret).To(BeNil())
		})

		It("returns when context is done", func() {
			ctx, cancel := context.WithCancel(ctx)
			cancel()

			err := util.WithTimeout(time.Second, util.ApplyContext(t.refreshMemory, ctx))
			Expect(err).To(MatchError(context.Canceled))
		})
	})

	Context("newMemoryFromHTTPSOs", func() {
		var (
			t *table
		)

		BeforeEach(func() {
			i, _ := NewTable(sharedInformerFactory, namespace)
			t = i.(*table)
		})

		It("returns new memory based on HTTPSOs", func() {
			for _, httpso := range httpsoList.Items {
				httpso := httpso

				key := *k8s.NamespacedNameFromObject(&httpso)
				t.httpScaledObjects[key] = &httpso
			}

			tm := t.newMemoryFromHTTPSOs()

			for _, httpso := range httpsoList.Items {
				namespacedName := k8s.NamespacedNameFromObject(&httpso)

				ret := tm.Recall(namespacedName)
				Expect(ret).To(Equal(&httpso))
			}
		})
	})
})
