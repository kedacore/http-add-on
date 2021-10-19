// Handlers contains the gRPC implementation for an external scaler as defined
// by the KEDA documentation at https://keda.sh/docs/2.0/concepts/external-scalers/#built-in-scalers-interface
// This is the interface KEDA will poll in order to get the request queue size
// and scale user apps properly
package main

import (
	context "context"
	"fmt"
	"math/rand"
	"time"

	"github.com/go-logr/logr"
	empty "github.com/golang/protobuf/ptypes/empty"
	"github.com/kedacore/http-add-on/pkg/routing"
	externalscaler "github.com/kedacore/http-add-on/proto"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

type impl struct {
	lggr                    logr.Logger
	pinger                  *queuePinger
	routingTable            routing.TableReader
	targetMetric            int64
	targetMetricInterceptor int64
	externalscaler.UnimplementedExternalScalerServer
}

func newImpl(
	lggr logr.Logger,
	pinger *queuePinger,
	routingTable routing.TableReader,
	defaultTargetMetric int64,
	defaultTargetMetricInterceptor int64,
) *impl {
	return &impl{
		lggr:                    lggr,
		pinger:                  pinger,
		routingTable:            routingTable,
		targetMetric:            defaultTargetMetric,
		targetMetricInterceptor: defaultTargetMetricInterceptor,
	}
}

func (e *impl) Ping(context.Context, *empty.Empty) (*empty.Empty, error) {
	return &empty.Empty{}, nil
}

func (e *impl) IsActive(
	ctx context.Context,
	scaledObject *externalscaler.ScaledObjectRef,
) (*externalscaler.IsActiveResponse, error) {
	lggr := e.lggr.WithName("IsActive")
	host, ok := scaledObject.ScalerMetadata["host"]
	if !ok {
		err := fmt.Errorf("no 'host' field found in ScaledObject metadata")
		lggr.Error(err, "returning immediately from IsActive RPC call", "ScaledObject", scaledObject)
		return nil, err
	}
	if host == "interceptor" {
		return &externalscaler.IsActiveResponse{
			Result: true,
		}, nil
	}
	allCounts := e.pinger.counts()
	hostCount, ok := allCounts[host]
	if !ok {
		err := fmt.Errorf("host '%s' not found in counts", host)
		lggr.Error(err, "Given host was not found in queue count map", "host", host, "allCounts", allCounts)
		return nil, err
	}
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
					"error getting active status in stream, continuing",
				)
				continue
			}
			server.Send(&externalscaler.IsActiveResponse{
				Result: active.Result,
			})
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
	var targetPendingRequests int64
	if host == "interceptor" {
		targetPendingRequests = e.targetMetricInterceptor
	} else {
		target, err := e.routingTable.Lookup(host)
		if err != nil {
			lggr.Error(
				err,
				"error getting target for host",
				"host",
				host,
			)
			return nil, err
		}
		targetPendingRequests = int64(target.TargetPendingRequests)
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
	allCounts := e.pinger.counts()
	hostCount, ok := allCounts[host]
	if !ok {
		if host == "interceptor" {
			hostCount = e.pinger.aggregate()
		} else {
			err := fmt.Errorf("host '%s' not found in counts", host)
			lggr.Error(err, "allCounts", allCounts)
			return nil, err
		}
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
