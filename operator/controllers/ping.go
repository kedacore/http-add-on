package controllers

import (
	"context"
	"fmt"
	"net/http"

	"github.com/kedacore/http-add-on/pkg/k8s"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func pingInterceptors(
	ctx context.Context,
	cl client.Client,
	httpCl *http.Client,
	ns,
	interceptorSvcName,
	interceptorPort string,
) error {
	endpointURLs, err := k8s.EndpointsForService(
		ctx,
		ns,
		interceptorSvcName,
		interceptorPort,
		k8s.EndpointsFuncForControllerClient(cl),
	)
	if err != nil {
		return errors.Wrap(err, "pingInterceptors")
	}
	errGrp, _ := errgroup.WithContext(ctx)
	for _, endpointURL := range endpointURLs {
		endpointStr := endpointURL.String()
		errGrp.Go(func() error {
			fullAddr := fmt.Sprintf("%s/routing_ping", endpointStr)
			_, err := httpCl.Get(fullAddr)
			return err
		})
	}
	return errGrp.Wait()
}
