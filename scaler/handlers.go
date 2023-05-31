// Handlers contains the gRPC implementation for an external scaler as defined
// by the KEDA documentation at https://keda.sh/docs/2.0/concepts/external-scalers/#built-in-scalers-interface
// This is the interface KEDA will poll in order to get the request queue size
// and scale user apps properly
package main

import (
	"context"
	"math/rand"
	"time"

	"github.com/go-logr/logr"
	"github.com/kedacore/keda/v2/pkg/scalers/externalscaler"
	"google.golang.org/protobuf/types/known/emptypb"
	"k8s.io/utils/pointer"

	informershttpv1alpha1 "github.com/kedacore/http-add-on/operator/generated/informers/externalversions/http/v1alpha1"
	"github.com/kedacore/http-add-on/pkg/k8s"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

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
	namespacedName := k8s.NamespacedNameFromScaledObjectRef(scaledObject)

	key := namespacedName.String()
	count := e.pinger.counts()[key]

	active := count > 0
	res := &externalscaler.IsActiveResponse{
		Result: active,
	}
	return res, nil
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

	namespacedName := k8s.NamespacedNameFromScaledObjectRef(sor)
	metricName := MetricName(namespacedName)

	httpso, err := e.httpsoInformer.Lister().HTTPScaledObjects(sor.Namespace).Get(sor.Name)
	if err != nil {
		lggr.Error(err, "unable to get HTTPScaledObject", "name", sor.Name, "namespace", sor.Namespace)
		return nil, err
	}
	targetPendingRequests := int64(pointer.Int32Deref(httpso.Spec.TargetPendingRequests, 100))

	res := &externalscaler.GetMetricSpecResponse{
		MetricSpecs: []*externalscaler.MetricSpec{
			{
				MetricName: metricName,
				TargetSize: targetPendingRequests,
			},
		},
	}
	return res, nil
}

func (e *impl) GetMetrics(
	_ context.Context,
	metricRequest *externalscaler.GetMetricsRequest,
) (*externalscaler.GetMetricsResponse, error) {
	sor := metricRequest.ScaledObjectRef

	namespacedName := k8s.NamespacedNameFromScaledObjectRef(sor)
	metricName := MetricName(namespacedName)

	key := namespacedName.String()
	count := e.pinger.counts()[key]

	res := &externalscaler.GetMetricsResponse{
		MetricValues: []*externalscaler.MetricValue{
			{
				MetricName:  metricName,
				MetricValue: int64(count),
			},
		},
	}
	return res, nil
}
