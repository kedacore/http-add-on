package e2e

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestE2ELoad(t *testing.T) {
	r := require.New(t)
	h, shouldRun, err := setup()
	if !shouldRun {
		t.Logf("not running %s", t.Name())
		t.SkipNow()
	}
	defer h.close()
	r.NoError(err)
	ns := h.cfg.namespace()
	t.Logf("Running %s in namespace %s", t.Name(), ns)

	cl, restCfg, err := getClient()
	r.NoError(err)

	// ensure that the interceptor and XKCD scaledobjects
	// exist
	_, err = getScaledObject(h, cl, ns, "keda-add-ons-http-interceptor")
	r.NoError(err)
	_, err = getScaledObject(h, cl, ns, "xkcd-app")
	r.NoError(err)

	// ensure that there are no replicas of the xkcd service
	scalerAdminCl, err := getProxiedHTTPCl(
		h,
		cl,
		restCfg,
		h.cfg.ScalerAdminSvc,
	)
	r.NoError(err)
	res, err := scalerAdminCl.Get("/queue")
	r.NoError(err)
	res.Body.Close()
	counts := map[string]int{}
	resBytes, err := io.ReadAll(res.Body)
	r.NoError(err)
	r.NoError(json.Unmarshal(resBytes, &counts))
	for host, count := range counts {
		r.True(
			count == 0,
			"count for host %s was %d, expected 0",
			host,
			count,
		)
	}

	// issue a first request to the XKCD service to
	// make sure it scales up from zero in reasonable
	// time
	ingURL, err := url.Parse(h.cfg.IngAddress)
	r.NoError(err)
	start := time.Now()
	res, err = http.Get(ingURL.String())
	r.NoError(err)
	const maxScaleDur = 2 * time.Second
	r.Less(
		time.Since(start),
		maxScaleDur,
		"didn't scale up within %s", maxScaleDur,
	)
	r.Equal(200, res.Status)

	r.NoError(makeRequests(
		h,
		http.DefaultClient,
		ingURL,
		h.cfg.NumReqsAgainstProxy,
		func() error {
			if err := checkInterceptorMetrics(
				h,
				restCfg,
				ns,
				h.cfg.ProxyAdminSvc,
				h.cfg.ProxyAdminPort,
			); err != nil {
				return err
			}

			if err := checkScalerMetrics(
				h,
				restCfg,
				ns,
				h.cfg.ScalerAdminSvc,
				h.cfg.ScalerAdminPort,
			); err != nil {
				return err
			}
			return nil
		},
		h.cfg.AdminServerCheckDur,
	))
}
