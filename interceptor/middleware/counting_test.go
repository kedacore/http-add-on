package middleware

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	httpv1alpha1 "github.com/kedacore/http-add-on/operator/apis/http/v1alpha1"
	"github.com/kedacore/http-add-on/pkg/k8s"
	"github.com/kedacore/http-add-on/pkg/queue"
	"github.com/kedacore/http-add-on/pkg/util"
)

func TestCountMiddleware(t *testing.T) {
	r := require.New(t)

	uri, err := url.Parse("https://testingkeda.com")
	r.NoError(err)

	httpso := &httpv1alpha1.HTTPScaledObject{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test-namespace",
		},
		Spec: httpv1alpha1.HTTPScaledObjectSpec{
			ScaleTargetRef: httpv1alpha1.ScaleTargetRef{
				Name:    "testdepl",
				Service: "testservice",
				Port:    8080,
			},
			TargetPendingRequests: ptr.To[int32](123),
		},
	}
	namespacedName := k8s.NamespacedNameFromObject(httpso).String()

	queueCounter := queue.NewFakeCounterBuffered()

	var concurrencyDuringRequest int
	middleware := NewCountingMiddleware(
		queueCounter,
		http.HandlerFunc(func(wr http.ResponseWriter, _ *http.Request) {
			counts, err := queueCounter.Current()
			if err == nil {
				concurrencyDuringRequest = counts.Counts[namespacedName].Concurrency
			}
			wr.WriteHeader(200)
			_, _ = wr.Write([]byte("OK"))
		}),
	)

	req, err := http.NewRequest("GET", "/something", nil)
	r.NoError(err)
	reqCtx := req.Context()
	reqCtx = util.ContextWithLogger(reqCtx, logr.Discard())
	reqCtx = util.ContextWithHTTPSO(reqCtx, httpso)
	req = req.WithContext(reqCtx)
	req.Host = uri.Host

	respRecorder := httptest.NewRecorder()
	middleware.ServeHTTP(respRecorder, req)

	r.Equal(http.StatusOK, respRecorder.Code)
	r.Equal("OK", respRecorder.Body.String())

	// During the request, concurrency should have been 1
	r.Equal(1, concurrencyDuringRequest)

	// After the request completes, concurrency should be back to 0
	counts, err := queueCounter.Current()
	r.NoError(err)
	r.Equal(0, counts.Counts[namespacedName].Concurrency)
}
