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
	externalscaler "github.com/kedacore/http-add-on/scaler/gen"
	"google.golang.org/protobuf/types/known/emptypb"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

type impl struct {
	q httpQueue
}

func newImpl(q httpQueue) *impl {
	return &impl{q: q}
}

func (e *impl) Ping(context.Context, *empty.Empty) (*empty.Empty, error) {
	return &empty.Empty{}, nil
}

func (e *impl) IsActive(ctx context.Context, scaledObject *externalscaler.ScaledObjectRef) (*externalscaler.IsActiveResponse, error) {
	return &externalscaler.IsActiveResponse{
		Result: true,
	}, nil
}

func (e *impl) GetMetricSpec(_ context.Context, sor *externalscaler.ScaledObjectRef) (*externalscaler.GetMetricSpecResponse, error) {
	return &externalscaler.GetMetricSpecResponse{
		MetricSpecs: []*externalscaler.MetricSpec{
			{
				MetricName: "queueSize",
				TargetSize: 100,
			},
		},
	}, nil
}

func (e *impl) GetMetrics(_ context.Context, metricRequest *externalscaler.GetMetricsRequest) (*externalscaler.GetMetricsResponse, error) {
	return &externalscaler.GetMetricsResponse{
		MetricValues: []*externalscaler.MetricValue{
			{
				MetricName:  "queueSize",
				MetricValue: int64(e.q.pendingCounter()),
			},
		},
	}, nil
}

func (e *impl) New(_ context.Context, nr *externalscaler.NewRequest) (*empty.Empty, error) {
	return &empty.Empty{}, nil // not needed
}

func (e *impl) Close(_ context.Context, sor *externalscaler.ScaledObjectRef) (*emptypb.Empty, error) {
	return &empty.Empty{}, nil // not needed
}
