// Handlers contains the gRPC implementation for an external scaler as defined
// by the KEDA documentation at https://keda.sh/docs/2.0/concepts/external-scalers/#built-in-scalers-interface
// This is the interface KEDA will poll in order to get the request queue size
// and scale user apps properly
package main

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/go-logr/logr"
	"github.com/kedacore/keda/v2/pkg/scalers/externalscaler"
	"google.golang.org/protobuf/types/known/emptypb"
	"k8s.io/utils/ptr"

	informershttpv1alpha1 "github.com/kedacore/http-add-on/operator/generated/informers/externalversions/http/v1alpha1"
	"github.com/kedacore/http-add-on/pkg/k8s"
	"github.com/kedacore/http-add-on/pkg/util"
)

const (
	keyInterceptorTargetPendingRequests = "interceptorTargetPendingRequests"
)

var streamInterval time.Duration

func init() {
	defaultMS := 200
	timeoutMS, err := util.ResolveOsEnvInt("KEDA_HTTP_SCALER_STREAM_INTERVAL_MS", defaultMS)
	if err != nil {
		timeoutMS = defaultMS
	}
	streamInterval = time.Duration(timeoutMS) * time.Millisecond
}

type impl struct {
	lggr           logr.Logger
	pinger         *queuePinger
	httpsoInformer informershttpv1alpha1.HTTPScaledObjectInformer
	targetMetric   int64
	externalscaler.UnimplementedExternalScalerServer
}

func newImpl(
	lggr logr.Logger,
	pinger *queuePinger,
	httpsoInformer informershttpv1alpha1.HTTPScaledObjectInformer,
	defaultTargetMetric int64,
) *impl {
	return &impl{
		lggr:           lggr,
		pinger:         pinger,
		httpsoInformer: httpsoInformer,
		targetMetric:   defaultTargetMetric,
	}
}

func (e *impl) Ping(context.Context, *emptypb.Empty) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (e *impl) IsActive(
	ctx context.Context,
	sor *externalscaler.ScaledObjectRef,
) (*externalscaler.IsActiveResponse, error) {
	lggr := e.lggr.WithName("IsActive")

	gmr, err := e.GetMetrics(ctx, &externalscaler.GetMetricsRequest{
		ScaledObjectRef: sor,
	})
	if err != nil {
		lggr.Error(err, "GetMetrics failed", "scaledObjectRef", sor.String())
		return nil, err
	}

	metricValues := gmr.GetMetricValues()
	if err := errors.New("len(metricValues) != 1"); len(metricValues) != 1 {
		lggr.Error(err, "invalid GetMetricsResponse", "scaledObjectRef", sor.String(), "getMetricsResponse", gmr.String())
		return nil, err
	}
	metricValue := metricValues[0].GetMetricValue()

	active := metricValue > 0
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
	// we call server.Send (below) every streamInterval, which tells it to immediately
	// ping our IsActive RPC
	ticker := time.NewTicker(streamInterval)
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
		if scalerMetadata := sor.GetScalerMetadata(); scalerMetadata != nil {
			if interceptorTargetPendingRequests, ok := scalerMetadata[keyInterceptorTargetPendingRequests]; ok {
				return e.interceptorMetricSpec(metricName, interceptorTargetPendingRequests)
			}
		}

		lggr.Error(err, "unable to get HTTPScaledObject", "name", sor.Name, "namespace", sor.Namespace)
		return nil, err
	}
	targetPendingRequests := int64(ptr.Deref(httpso.Spec.TargetPendingRequests, 100))

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

func (e *impl) interceptorMetricSpec(metricName string, interceptorTargetPendingRequests string) (*externalscaler.GetMetricSpecResponse, error) {
	lggr := e.lggr.WithName("interceptorMetricSpec")

	targetPendingRequests, err := strconv.ParseInt(interceptorTargetPendingRequests, 10, 64)
	if err != nil {
		lggr.Error(err, "unable to parse interceptorTargetPendingRequests", "value", interceptorTargetPendingRequests)
		return nil, err
	}

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
	count := int64(e.pinger.counts()[key])

	if count == 0 {
		if scalerMetadata := sor.GetScalerMetadata(); scalerMetadata != nil {
			if _, ok := scalerMetadata[keyInterceptorTargetPendingRequests]; ok {
				return e.interceptorMetrics(metricName)
			}
		}
	}

	res := &externalscaler.GetMetricsResponse{
		MetricValues: []*externalscaler.MetricValue{
			{
				MetricName:  metricName,
				MetricValue: count,
			},
		},
	}
	return res, nil
}

func (e *impl) interceptorMetrics(metricName string) (*externalscaler.GetMetricsResponse, error) {
	lggr := e.lggr.WithName("interceptorMetrics")

	var count int64
	for _, v := range e.pinger.counts() {
		count += int64(v)
	}
	if err := strconv.ErrRange; count < 0 {
		lggr.Error(err, "count overflowed", "value", count)
		return nil, err
	}

	res := &externalscaler.GetMetricsResponse{
		MetricValues: []*externalscaler.MetricValue{
			{
				MetricName:  metricName,
				MetricValue: count,
			},
		},
	}
	return res, nil
}
