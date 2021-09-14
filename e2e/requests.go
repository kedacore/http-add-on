package e2e

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"golang.org/x/sync/errgroup"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/flowcontrol"
)

func makeRequests(
	ctx context.Context,
	httpCl *http.Client,
	addr *url.URL,
	numReqs int,
	checkFunc func() error,
	tickDur time.Duration,
) error {
	ctx, done := context.WithCancel(ctx)
	requestsErrGroup, ctx := errgroup.WithContext(ctx)
	req, err := http.NewRequest("GET", addr.String(), nil)
	if err != nil {
		return err
	}
	req = req.WithContext(ctx)
	for i := 0; i < numReqs; i++ {
		requestsErrGroup.Go(func() error {
			res, err := httpCl.Do(req)
			if err != nil {
				return err
			}
			if res.StatusCode != 200 {
				return fmt.Errorf(
					"expected status code 200, got %d",
					res.StatusCode,
				)
			}
			return nil
		})
	}

	tickErrGroup, ctx := errgroup.WithContext(ctx)
	ticker := time.NewTicker(tickDur)
	defer ticker.Stop()
	tickErrGroup.Go(func() error {
		for {
			select {
			case <-ticker.C:
				if err := checkFunc(); err != nil {
					return err
				}
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	})

	requestsErr := requestsErrGroup.Wait()
	done()
	tickErr := tickErrGroup.Wait()
	if requestsErr != nil {
		return requestsErr
	}
	if tickErr != nil {
		return tickErr
	}
	return nil
}

func makeProxiedRequestsToSvc(
	ctx context.Context,
	cfg *rest.Config,
	ns,
	svcName string,
	svcPort,
	numReqs int,
) error {
	cls, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return err
	}
	restCl := cls.CoreV1().RESTClient()
	makeReq := func(ctx context.Context) error {
		req := restCl.
			Get().
			Namespace(ns).
			Resource("services").
			Name(fmt.Sprintf("%s:%d", svcName, svcPort)).
			SubResource("proxy").
			Throttle(flowcontrol.NewFakeAlwaysRateLimiter())
		res := req.Do(ctx)
		if res.Error() != nil {
			return res.Error()
		}
		code := respCode(res)
		if code != 200 {
			return fmt.Errorf(
				"unexpected status code: %d",
				code,
			)
		}
		return nil
	}
	g, ctx := errgroup.WithContext(ctx)
	for i := 0; i < numReqs; i++ {
		g.Go(func() error {
			return makeReq(ctx)
		})
	}
	return g.Wait()
}

func respCode(res rest.Result) int {
	code := 200
	res.StatusCode(&code)
	return code
}
