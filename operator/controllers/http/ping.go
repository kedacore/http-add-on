package http

import (
	"context"
	"fmt"
	"net/http"

	"golang.org/x/sync/errgroup"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kedacore/http-add-on/pkg/k8s"
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
		return fmt.Errorf("pingInterceptors: %w", err)
	}
	errGrp, _ := errgroup.WithContext(ctx)
	for _, endpointURL := range endpointURLs {
		endpointStr := endpointURL.String()
		errGrp.Go(func() error {
			fullAddr := fmt.Sprintf("%s/routing_ping", endpointStr)
			resp, err := httpCl.Get(fullAddr)
			if err != nil {
				return err
			}
			resp.Body.Close()
			return nil
		})
	}
	return errGrp.Wait()
}
