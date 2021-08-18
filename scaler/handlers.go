// Handlers contains the gRPC implementation for an external scaler as defined
// by the KEDA documentation at https://keda.sh/docs/2.0/concepts/external-scalers/#built-in-scalers-interface
// This is the interface KEDA will poll in order to get the request queue size
// and scale user apps properly
package main

import (
	context "context"
	"math/rand"
	"time"

	empty "github.com/golang/protobuf/ptypes/empty"
	externalscaler "github.com/kedacore/http-add-on/proto"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

type impl struct {
	pinger       *queuePinger
	targetMetric int64
	externalscaler.UnimplementedExternalScalerServer
}

func newImpl(pinger *queuePinger, targetMetric int64) *impl {
	return &impl{pinger: pinger, targetMetric: targetMetric}
}

func (e *impl) Ping(context.Context, *empty.Empty) (*empty.Empty, error) {
	return &empty.Empty{}, nil
}

func (e *impl) IsActive(
	ctx context.Context,
	scaledObject *externalscaler.ScaledObjectRef,
) (*externalscaler.IsActiveResponse, error) {
	return &externalscaler.IsActiveResponse{
		Result: true,
	}, nil
}

func (e *impl) StreamIsActive(
	in *externalscaler.ScaledObjectRef,
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
			server.Send(&externalscaler.IsActiveResponse{
				Result: true,
			})
		}
	}
}

func (e *impl) GetMetricSpec(
	_ context.Context,
	sor *externalscaler.ScaledObjectRef,
) (*externalscaler.GetMetricSpecResponse, error) {
	targetMetricValue := e.targetMetric
	return &externalscaler.GetMetricSpecResponse{
		MetricSpecs: []*externalscaler.MetricSpec{
			{
				MetricName: "queueSize",
				TargetSize: targetMetricValue,
			},
		},
	}, nil
}

func (e *impl) GetMetrics(
	_ context.Context,
	metricRequest *externalscaler.GetMetricsRequest,
) (*externalscaler.GetMetricsResponse, error) {
	size := int64(e.pinger.count())
	return &externalscaler.GetMetricsResponse{
		MetricValues: []*externalscaler.MetricValue{
			{
				MetricName:  "queueSize",
				MetricValue: size,
			},
		},
	}, nil
}
