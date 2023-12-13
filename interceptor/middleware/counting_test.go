package middleware

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
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
			Namespace: "@",
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

	queueCounter := queue.NewFakeCounter()

	middleware := NewCountingMiddleware(
		queueCounter,
		http.HandlerFunc(func(wr http.ResponseWriter, req *http.Request) {
			wr.WriteHeader(200)
			_, err := wr.Write([]byte("OK"))
			r.NoError(err)
		}),
	)

	ctx := context.Background()

	// for a valid request, we expect the queue to be resized twice.
	// once to mark a pending HTTP request, then a second time to remove it.
	// by the end of both sends, resize1 + resize2 should be 0,
	// or in other words, the queue size should be back to zero

	// run middleware with the host in the request
	req, err := http.NewRequest("GET", "/something", nil)
	r.NoError(err)
	reqCtx := req.Context()
	reqCtx = util.ContextWithLogger(reqCtx, logr.Discard())
	reqCtx = util.ContextWithHTTPSO(reqCtx, httpso)
	req = req.WithContext(reqCtx)
	req.Host = uri.Host

	agg, respRecorder := expectResizes(
		ctx,
		t,
		2,
		middleware,
		req,
		queueCounter,
		func(t *testing.T, hostAndCount queue.HostAndCount) {
			t.Helper()
			r := require.New(t)
			r.Equal(float64(1), math.Abs(float64(hostAndCount.Count)))
			r.Equal(namespacedName, hostAndCount.Host)
		},
	)
	r.Equal(http.StatusOK, respRecorder.Code)
	r.Equal(http.StatusText(respRecorder.Code), respRecorder.Body.String())
	r.Equal(0, agg)
}

// expectResizes creates a new httptest.ResponseRecorder, then passes req through
// the middleware. every time the middleware calls fakeCounter.Resize(), it calls
// resizeCheckFn with t and the queue.HostCount that represents the resize call
// that was made. it also maintains an aggregate delta of the counts passed to
// Resize. If, for example, the following integers were passed to resize over
// 4 calls: [-1, 1, 1, 2], the aggregate would be -1+1+1+2=3
//
// this function returns the aggregate and the httptest.ResponseRecorder that was
// created and used with the middleware
func expectResizes(
	ctx context.Context,
	t *testing.T,
	nResizes int,
	middleware http.Handler,
	req *http.Request,
	fakeCounter *queue.FakeCounter,
	resizeCheckFn func(*testing.T, queue.HostAndCount),
) (int, *httptest.ResponseRecorder) {
	t.Helper()
	r := require.New(t)
	const timeout = 1 * time.Second
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	grp, ctx := errgroup.WithContext(ctx)
	agg := 0
	grp.Go(func() error {
		// we expect the queue to be resized nResizes times
		for i := 0; i < nResizes; i++ {
			select {
			case hostAndCount := <-fakeCounter.ResizedCh:
				agg += hostAndCount.Count
				resizeCheckFn(t, hostAndCount)
			case <-ctx.Done():
				return fmt.Errorf(
					"timed out waiting for the count middleware. expected %d resizes, timeout was %s, iteration %d",
					nResizes,
					timeout,
					i,
				)
			}
		}
		return nil
	})

	respRecorder := httptest.NewRecorder()
	middleware.ServeHTTP(respRecorder, req)

	r.NoError(grp.Wait())

	return agg, respRecorder
}
