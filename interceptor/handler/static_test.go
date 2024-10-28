package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"

	"github.com/go-logr/logr/funcr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kedacore/http-add-on/pkg/routing"
	"github.com/kedacore/http-add-on/pkg/util"
)

var _ = Describe("ServeHTTP", func() {
	var (
		w *httptest.ResponseRecorder
		r *http.Request

		sc = http.StatusTeapot
		st = http.StatusText(sc)

		se = errors.New("test error")

		rh = ""
	)

	BeforeEach(func() {
		w = httptest.NewRecorder()
		r = httptest.NewRequest(http.MethodGet, "/", nil)
	})

	It("serves expected status code and body", func() {
		sh := NewStatic(sc, nil)
		sh.ServeHTTP(w, r)

		Expect(w.Code).To(Equal(sc))
		Expect(w.Body.String()).To(Equal(st))
	})

	It("logs the failed request", func() {
		var b bool
		r = r.WithContext(util.ContextWithLogger(r.Context(), funcr.NewJSON(
			func(obj string) {
				var m map[string]interface{}

				err := json.Unmarshal([]byte(obj), &m)
				Expect(err).NotTo(HaveOccurred())

				rk := routing.NewKeyFromRequest(r, rh)
				Expect(m).To(HaveKeyWithValue("error", se.Error()))
				Expect(m).To(HaveKeyWithValue("msg", st))
				Expect(m).To(HaveKeyWithValue("routingKey", rk.String()))

				b = true
			},
			funcr.Options{},
		)))

		sh := NewStatic(sc, se)
		sh.ServeHTTP(w, r)

		Expect(b).To(BeTrue())
	})
})
