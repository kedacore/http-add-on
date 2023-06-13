package routing

import (
	"fmt"
	"net/http"
	"net/url"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	httpv1alpha1 "github.com/kedacore/http-add-on/operator/apis/http/v1alpha1"
)

var _ = Describe("Key", func() {
	Context("New", func() {
		const (
			host0 = "kubernetes.io"
			host1 = "kubernetes.io:443"
			path0 = "abc/def"
			path1 = "abc/def/"
			path2 = "abc/def//"
			path3 = "/abc/def"
			path4 = "/abc/def/"
			path5 = "/abc/def//"
			path6 = "//abc/def"
			path7 = "//abc/def/"
			path8 = "//abc/def//"
			norm0 = "///"
			norm1 = "//kubernetes.io/"
			norm2 = "///abc/def/"
			norm3 = "//kubernetes.io/abc/def/"
		)

		It("returns expected key for blank host and blank path", func() {
			key := NewKey("", "")
			Expect(key).To(Equal(Key(norm0)))
		})

		It("returns expected key for host without port", func() {
			key := NewKey(host0, "")
			Expect(key).To(Equal(Key(norm1)))
		})

		It("returns expected key for host with port", func() {
			key := NewKey(host1, "")
			Expect(key).To(Equal(Key(norm1)))
		})

		It("returns expected key for path with no leading slashes and no trailing slashes", func() {
			key := NewKey("", path0)
			Expect(key).To(Equal(Key(norm2)))
		})

		It("returns expected key for path with no leading slashes and single trailing slash", func() {
			key := NewKey("", path1)
			Expect(key).To(Equal(Key(norm2)))
		})

		It("returns expected key for path with no leading slashes and multiple trailing slashes", func() {
			key := NewKey("", path2)
			Expect(key).To(Equal(Key(norm2)))
		})

		It("returns expected key for path with single leading slashes and no trailing slashes", func() {
			key := NewKey("", path3)
			Expect(key).To(Equal(Key(norm2)))
		})

		It("returns expected key for path with single leading slash and single trailing slash", func() {
			key := NewKey("", path4)
			Expect(key).To(Equal(Key(norm2)))
		})

		It("returns expected key for path with single leading slash and multiple trailing slashes", func() {
			key := NewKey("", path5)
			Expect(key).To(Equal(Key(norm2)))
		})

		It("returns expected key for path with multiple leading slashes and no trailing slashes", func() {
			key := NewKey("", path6)
			Expect(key).To(Equal(Key(norm2)))
		})

		It("returns expected key for path with multiple leading slash and single trailing slash", func() {
			key := NewKey("", path7)
			Expect(key).To(Equal(Key(norm2)))
		})

		It("returns expected key for path with multiple leading slash and multiple trailing slashes", func() {
			key := NewKey("", path8)
			Expect(key).To(Equal(Key(norm2)))
		})

		It("returns expected key for non-blank host and non-blank path", func() {
			key := NewKey(host1, path8)
			Expect(key).To(Equal(Key(norm3)))
		})

		It("returns nil for nil HTTPSO", func() {
			key := NewKeysFromHTTPSO(nil)
			Expect(key).To(BeNil())
		})
	})

	Context("NewFromURL", func() {
		It("returns expected key for URL", func() {
			const (
				host = "kubernetes.io"
				path = "abc/def"
				norm = "//kubernetes.io/abc/def/"
			)

			url, err := url.Parse(fmt.Sprintf("https://%s:443/%s?123=456#789", host, path))
			Expect(err).NotTo(HaveOccurred())
			Expect(url).NotTo(BeNil())

			key := NewKeyFromURL(url)
			Expect(key).To(Equal(Key(norm)))
		})

		It("returns nil for nil URL", func() {
			key := NewKeyFromURL(nil)
			Expect(key).To(BeNil())
		})
	})

	Context("NewFromRequest", func() {
		It("returns expected key for Request", func() {
			const (
				host = "kubernetes.io"
				path = "abc/def"
				norm = "//kubernetes.io/abc/def/"
			)

			r, err := http.NewRequest("GET", fmt.Sprintf("https://%s:443/%s?123=456#789", host, path), nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(r).NotTo(BeNil())

			key := NewKeyFromRequest(r)
			Expect(key).To(Equal(Key(norm)))
		})

		It("returns nil for nil Request", func() {
			key := NewKeyFromRequest(nil)
			Expect(key).To(BeNil())
		})
	})

	Context("String", func() {
		const (
			host = "kubernetes.io"
			path = "abc/def"
			norm = "//kubernetes.io/abc/def/"
		)

		It("returns expected string for key", func() {
			key := NewKey(host, path)
			Expect(key).NotTo(BeNil())
			Expect(key.String()).To(Equal(norm))
		})

		It("returns expected string for key with printf", func() {
			key := NewKey(host, path)
			Expect(key).NotTo(BeNil())

			str := fmt.Sprintf("%v", key)
			Expect(str).To(Equal(norm))
		})
	})
})

var _ = Describe("Keys", func() {
	Context("New", func() {
		It("returns expected key for HTTPSO", func() {
			const (
				host = "kubernetes.io"
				// TODO(pedrotorres): delete this when we support path prefix
				norm = "//kubernetes.io/"
				// TODO(pedrotorres): uncomment this when we support path prefix
				// path = "abc/def"
				// norm = "//kubernetes.io/abc/def/"
			)

			keys := NewKeysFromHTTPSO(&httpv1alpha1.HTTPScaledObject{
				Spec: httpv1alpha1.HTTPScaledObjectSpec{
					Hosts: []string{
						host,
					},
					// TODO(pedrotorres): uncomment this when we support path prefix
					// PathPrefix: path,
				},
			})
			Expect(keys).To(ConsistOf(Keys{
				Key(norm),
			}))
		})

		It("returns expected keys for HTTPSO", func() {
			const (
				host0 = "keda.sh"
				host1 = "kubernetes.io"
				// TODO(pedrotorres): delete this when we support path prefix
				norm0 = "//keda.sh/"
				norm1 = "//kubernetes.io/"
				// TODO(pedrotorres): uncomment this when we support path prefix
				// path0 = "abc/def"
				// path1 = "123/456"
				// norm0 = "//kubernetes.io/abc/def/"
				// norm1 = "//keda.sh/123/456/"
			)

			keys := NewKeysFromHTTPSO(&httpv1alpha1.HTTPScaledObject{
				Spec: httpv1alpha1.HTTPScaledObjectSpec{
					Hosts: []string{
						host0,
						host1,
					},
					// TODO(pedrotorres): uncomment this when we support path prefix
					// PathPrefix: path,
				},
			})
			Expect(keys).To(ConsistOf(Keys{
				Key(norm0),
				Key(norm1),
			}))
		})

		It("returns nil for nil HTTPSO", func() {
			key := NewKeysFromHTTPSO(nil)
			Expect(key).To(BeNil())
		})
	})
})
