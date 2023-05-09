// Handlers contains the gRPC implementation for an external scaler as defined
// by the KEDA documentation at https://keda.sh/docs/2.0/concepts/external-scalers/#built-in-scalers-interface
// This is the interface KEDA will poll in order to get the request queue size
// and scale user apps properly
package main

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/go-logr/logr"
	"google.golang.org/protobuf/types/known/emptypb"
	"k8s.io/utils/pointer"

	informershttpv1alpha1 "github.com/kedacore/http-add-on/operator/generated/informers/externalversions/http/v1alpha1"
	externalscaler "github.com/kedacore/http-add-on/proto"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

const (
	interceptor = "interceptor"
)

type impl struct {
	lggr                    logr.Logger
	pinger                  *queuePinger
	httpsoInformer          informershttpv1alpha1.HTTPScaledObjectInformer
	targetMetric            int64
	targetMetricInterceptor int64
	externalscaler.UnimplementedExternalScalerServer
}

func newImpl(
	lggr logr.Logger,
	pinger *queuePinger,
	httpsoInformer informershttpv1alpha1.HTTPScaledObjectInformer,
	defaultTargetMetric int64,
	defaultTargetMetricInterceptor int64,
) *impl {
	return &impl{
		lggr:                    lggr,
		pinger:                  pinger,
		httpsoInformer:          httpsoInformer,
		targetMetric:            defaultTargetMetric,
		targetMetricInterceptor: defaultTargetMetricInterceptor,
	}
}

func (e *impl) Ping(context.Context, *emptypb.Empty) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (e *impl) IsActive(
	_ context.Context,
	scaledObject *externalscaler.ScaledObjectRef,
) (*externalscaler.IsActiveResponse, error) {
	lggr := e.lggr.WithName("IsActive")
	host, ok := scaledObject.ScalerMetadata["host"]
	if !ok {
		err := fmt.Errorf("no 'host' field found in ScaledObject metadata")
		lggr.Error(err, "returning immediately from IsActive RPC call", "ScaledObject", scaledObject)
		return nil, err
	}
	if host == interceptor {
		return &externalscaler.IsActiveResponse{
			Result: true,
		}, nil
	}

	hostCount := e.pinger.counts()[host]
	active := hostCount > 0
	return &externalscaler.IsActiveResponse{
		Result: active,
	}, nil
}

func (e *impl) StreamIsActive(
	scaledObject *externalscaler.ScaledObjectRef,
	server externalscaler.ExternalScaler_StreamIsActiveServer,
) error {
	// this function communicates with KEDA via the 'server' parameter.
	// we call server.Send (below) every 200ms, which tells it to immediately
	// ping our IsActive RPC
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-server.Context().Done():
			return nil
		case <-ticker.C:
			active, err := e.IsActive(server.Context(), scaledObject)
			if err != nil {
				e.lggr.Error(
					err,
					"error getting active status in stream",
				)
				return err
			}
			err = server.Send(&externalscaler.IsActiveResponse{
				Result: active.Result,
			})
			if err != nil {
				e.lggr.Error(
					err,
					"error sending the active result in stream",
				)
				return err
			}
		}
	}
}

func (e *impl) GetMetricSpec(
	_ context.Context,
	sor *externalscaler.ScaledObjectRef,
) (*externalscaler.GetMetricSpecResponse, error) {
	lggr := e.lggr.WithName("GetMetricSpec")
	host, ok := sor.ScalerMetadata["host"]
	if !ok {
		err := fmt.Errorf("'host' not found in ScaledObject metadata")
		lggr.Error(err, "no 'host' found in ScaledObject metadata")
		return nil, err
	}
	targetPendingRequests := e.targetMetricInterceptor
	if host != interceptor {
		httpso, err := e.httpsoInformer.Lister().HTTPScaledObjects(sor.Namespace).Get(sor.Name)
		if err != nil {
			lggr.Error(err, "unable to get HTTPScaledObject", "name", sor.Name, "namespace", sor.Namespace)
			return nil, err
		}

		targetPendingRequests = int64(pointer.Int32Deref(httpso.Spec.TargetPendingRequests, 100))
	}
	metricSpecs := []*externalscaler.MetricSpec{
		{
			MetricName: host,
			TargetSize: targetPendingRequests,
		},
	}

	return &externalscaler.GetMetricSpecResponse{
		MetricSpecs: metricSpecs,
	}, nil
}

func (e *impl) GetMetrics(
	_ context.Context,
	metricRequest *externalscaler.GetMetricsRequest,
) (*externalscaler.GetMetricsResponse, error) {
	lggr := e.lggr.WithName("GetMetrics")
	host, ok := metricRequest.ScaledObjectRef.ScalerMetadata["host"]
	if !ok {
		err := fmt.Errorf("no 'host' field found in ScaledObject metadata")
		lggr.Error(err, "ScaledObjectRef", metricRequest.ScaledObjectRef)
		return nil, err
	}

	hostCount, ok := e.pinger.counts()[host]
	if !ok && host == interceptor {
		hostCount = e.pinger.aggregate()
	}
	metricValues := []*externalscaler.MetricValue{
		{
			MetricName:  host,
			MetricValue: int64(hostCount),
		},
	}
	return &externalscaler.GetMetricsResponse{
		MetricValues: metricValues,
	}, nil
}
