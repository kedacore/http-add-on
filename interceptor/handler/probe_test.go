package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/go-logr/logr/funcr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kedacore/http-add-on/pkg/util"
)

var _ = Describe("ProbeHandler", func() {
	Context("New", func() {
		It("returns new object with expected fields", func() {
			var (
				ctx = context.Background()
				ret = errors.New("test error")
			)

			var b bool
			healthCheckers := []util.HealthChecker{
				util.HealthCheckerFunc(func(_ context.Context) error {
					b = true

					return ret
				}),
			}

			ph := NewProbe(healthCheckers)
			Expect(ph).NotTo(BeNil())

			h := ph.healthy.Load()
			Expect(h).To(BeFalse())

			hcs := ph.healthCheckers
			Expect(hcs).To(HaveLen(1))

			hc := hcs[0]
			Expect(hc).NotTo(BeNil())

			err := hc.HealthCheck(ctx)
			Expect(err).To(MatchError(ret))

			Expect(b).To(BeTrue())
		})
	})

	Context("ServeHTTP", func() {
		const (
			host = "keda.sh"
			path = "/README"
		)

		var (
			w *httptest.ResponseRecorder
			r *http.Request
		)

		BeforeEach(func() {
			w = httptest.NewRecorder()

			r = httptest.NewRequest(http.MethodGet, path, nil)
			r.Host = host
		})

		When("healthy", func() {
			It("serves 200", func() {
				var (
					sc = http.StatusOK
					st = http.StatusText(sc)
				)

				var ph Probe
				ph.healthy.Store(true)

				ph.ServeHTTP(w, r)

				Expect(w.Code).To(Equal(sc))
				Expect(w.Body.String()).To(Equal(st))
			})
		})

		When("unhealthy", func() {
			It("serves 503", func() {
				var (
					sc = http.StatusServiceUnavailable
					st = http.StatusText(sc)
				)

				var ph Probe
				ph.healthy.Store(false)

				ph.ServeHTTP(w, r)

				Expect(w.Code).To(Equal(sc))
				Expect(w.Body.String()).To(Equal(st))
			})
		})
	})

	Context("Start", func() {
		It("returns when context is done", func() {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			var ph Probe
			err := util.WithTimeout(time.Second, func() error {
				ph.Start(ctx)

				return nil
			})
			Expect(err).NotTo(HaveOccurred())
		})

		It("invokes check every second", func() {
			const (
				n = 10
			)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			var i int
			ph := Probe{
				healthCheckers: []util.HealthChecker{
					util.HealthCheckerFunc(func(_ context.Context) error {
						i++

						return nil
					}),
				},
			}

			go ph.Start(ctx)

			time.Sleep(n * time.Second)

			Expect(i).To(BeNumerically("~", n, 2))
		})
	})

	Context("check", func() {
		When("all health checks succeed", func() {
			It("sets healthy to true", func() {
				var (
					ctx = context.Background()
				)

				var bs []bool
				ph := Probe{
					healthCheckers: []util.HealthChecker{
						util.HealthCheckerFunc(func(_ context.Context) error {
							bs = append(bs, true)
							return nil
						}),
						util.HealthCheckerFunc(func(_ context.Context) error {
							bs = append(bs, true)
							return nil
						}),
						util.HealthCheckerFunc(func(_ context.Context) error {
							bs = append(bs, true)
							return nil
						}),
					},
				}

				ph.check(ctx)

				healthCheckersLen := len(ph.healthCheckers)
				Expect(bs).To(HaveLen(healthCheckersLen))

				healthy := ph.healthy.Load()
				Expect(healthy).To(BeTrue())
			})
		})

		When("a health check fail", func() {
			It("sets healthy to false", func() {
				var (
					ctx = context.Background()
				)

				ph := Probe{
					healthCheckers: []util.HealthChecker{
						util.HealthCheckerFunc(func(_ context.Context) error {
							return nil
						}),
						util.HealthCheckerFunc(func(_ context.Context) error {
							return context.Canceled
						}),
						util.HealthCheckerFunc(func(_ context.Context) error {
							return nil
						}),
					},
				}
				ph.healthy.Store(true)

				ph.check(ctx)

				healthy := ph.healthy.Load()
				Expect(healthy).To(BeFalse())
			})

			It("logs the returned error", func() {
				const (
					msg = "health check function failed"
				)

				var (
					ctx = context.Background()
					ret = errors.New("test error")
				)

				var b bool
				ctx = util.ContextWithLogger(ctx, funcr.NewJSON(
					func(obj string) {
						var m map[string]interface{}

						err := json.Unmarshal([]byte(obj), &m)
						Expect(err).NotTo(HaveOccurred())

						Expect(m).To(HaveKeyWithValue("msg", msg))
						Expect(m).To(HaveKeyWithValue("error", ret.Error()))

						b = true
					},
					funcr.Options{},
				))

				ph := Probe{
					healthCheckers: []util.HealthChecker{
						util.HealthCheckerFunc(func(_ context.Context) error {
							return ret
						}),
					},
				}

				ph.check(ctx)

				Expect(b).To(BeTrue())
			})
		})
	})
})
