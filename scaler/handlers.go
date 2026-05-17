// Handlers contains the gRPC implementation for an external scaler as defined
// by the KEDA documentation at https://keda.sh/docs/2.0/concepts/external-scalers/#built-in-scalers-interface
// This is the interface KEDA will poll in order to get the request queue size
// and scale user apps properly
package main

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/go-logr/logr"
	"github.com/kedacore/keda/v2/pkg/scalers/externalscaler"
	"google.golang.org/protobuf/types/known/emptypb"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	httpv1beta1 "github.com/kedacore/http-add-on/operator/apis/http/v1beta1"
	"github.com/kedacore/http-add-on/pkg/k8s"
)

var errNoMetricValues = errors.New("no metric values in response")

type scalerHandler struct {
	lggr           logr.Logger
	pinger         *queuePinger
	reader         client.Reader
	streamInterval time.Duration
	externalscaler.UnimplementedExternalScalerServer
}

func newScalerHandler(lggr logr.Logger, pinger *queuePinger, reader client.Reader, streamInterval time.Duration) *scalerHandler {
	return &scalerHandler{
		lggr:           lggr,
		pinger:         pinger,
		reader:         reader,
		streamInterval: streamInterval,
	}
}

func (e *scalerHandler) Ping(context.Context, *emptypb.Empty) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (e *scalerHandler) IsActive(ctx context.Context, sor *externalscaler.ScaledObjectRef) (*externalscaler.IsActiveResponse, error) {
	lggr := e.lggr.WithName("IsActive")

	gmr, err := e.GetMetrics(ctx, &externalscaler.GetMetricsRequest{
		ScaledObjectRef: sor,
	})
	if err != nil {
		lggr.Error(err, "GetMetrics failed", "scaledObjectRef", sor.String())
		return nil, err
	}

	metricValues := gmr.GetMetricValues()
	if len(metricValues) == 0 {
		lggr.Error(errNoMetricValues, "invalid GetMetricsResponse", "scaledObjectRef", sor.String())
		return nil, errNoMetricValues
	}

	active := slices.ContainsFunc(metricValues, func(v *externalscaler.MetricValue) bool {
		return v.MetricValueFloat > 0 || v.MetricValue > 0
	})

	return &externalscaler.IsActiveResponse{Result: active}, nil
}

func (e *scalerHandler) StreamIsActive(scaledObject *externalscaler.ScaledObjectRef, server externalscaler.ExternalScaler_StreamIsActiveServer) error {
	// this function communicates with KEDA via the 'server' parameter.
	// we call server.Send (below) every streamInterval, which tells it to immediately
	// ping our IsActive RPC
	ticker := time.NewTicker(e.streamInterval)
	defer ticker.Stop()
	for {
		select {
		case <-server.Context().Done():
			return nil
		case <-ticker.C:
			active, err := e.IsActive(server.Context(), scaledObject)
			if err != nil {
				e.lggr.Error(err, "error getting active status in stream")
				return err
			}
			err = server.Send(&externalscaler.IsActiveResponse{
				Result: active.Result,
			})
			if err != nil {
				e.lggr.Error(err, "error sending the active result in stream")
				return err
			}
		}
	}
}

func (e *scalerHandler) GetMetricSpec(ctx context.Context, sor *externalscaler.ScaledObjectRef) (*externalscaler.GetMetricSpecResponse, error) {
	lggr := e.lggr.WithName("GetMetricSpec")

	scalerMetadata := sor.GetScalerMetadata()

	if irName, ok := scalerMetadata[k8s.InterceptorRouteKey]; ok {
		nn := types.NamespacedName{Namespace: sor.Namespace, Name: irName}
		var ir httpv1beta1.InterceptorRoute
		if err := e.reader.Get(ctx, nn, &ir); err != nil {
			lggr.Error(err, "failed to get InterceptorRoute", "namespace", sor.Namespace, "scaledObjectName", sor.Name, "interceptorRouteName", irName)
			return nil, err
		}

		var metricSpecs []*externalscaler.MetricSpec
		if m := ir.Spec.ScalingMetric.Concurrency; m != nil {
			metricSpecs = append(metricSpecs, &externalscaler.MetricSpec{
				MetricName:      ConcurrencyMetricName(irName),
				TargetSizeFloat: float64(m.TargetValue),
			})
		}
		if m := ir.Spec.ScalingMetric.RequestRate; m != nil {
			metricSpecs = append(metricSpecs, &externalscaler.MetricSpec{
				MetricName:      RateMetricName(irName),
				TargetSizeFloat: float64(m.TargetValue),
			})
		}

		return &externalscaler.GetMetricSpecResponse{
			MetricSpecs: metricSpecs,
		}, nil
	}

	err := fmt.Errorf("unable to get InterceptorRoute reference")
	lggr.Error(err, "unable to get the linked InterceptorRoute for ScaledObject", "name", sor.Name, "namespace", sor.Namespace)
	return nil, err
}

func (e *scalerHandler) GetMetrics(ctx context.Context, metricRequest *externalscaler.GetMetricsRequest) (*externalscaler.GetMetricsResponse, error) {
	lggr := e.lggr.WithName("GetMetrics")
	sor := metricRequest.ScaledObjectRef

	scalerMetadata := sor.GetScalerMetadata()

	if irName, ok := scalerMetadata[k8s.InterceptorRouteKey]; ok {
		nn := types.NamespacedName{Namespace: sor.Namespace, Name: irName}
		var ir httpv1beta1.InterceptorRoute
		if err := e.reader.Get(ctx, nn, &ir); err != nil {
			lggr.Error(err, "failed to get InterceptorRoute", "namespace", sor.Namespace, "scaledObjectName", sor.Name, "interceptorRouteName", irName)
			return nil, err
		}

		key := k8s.ResourceKeyFromNamespacedName(nn)

		if rr := ir.Spec.ScalingMetric.RequestRate; rr != nil {
			e.pinger.UpdateBucketConfig(key, rr.Window.Duration, rr.Granularity.Duration)
		}

		count := e.pinger.count(key)

		var metricValues []*externalscaler.MetricValue
		if ir.Spec.ScalingMetric.Concurrency != nil {
			metricValues = append(metricValues, &externalscaler.MetricValue{
				MetricName:       ConcurrencyMetricName(irName),
				MetricValueFloat: float64(count.Concurrency),
			})
		}
		if ir.Spec.ScalingMetric.RequestRate != nil {
			metricValues = append(metricValues, &externalscaler.MetricValue{
				MetricName:       RateMetricName(irName),
				MetricValueFloat: count.RequestRate,
			})
		}

		return &externalscaler.GetMetricsResponse{
			MetricValues: metricValues,
		}, nil
	}

	err := fmt.Errorf("unable to get InterceptorRoute reference")
	lggr.Error(err, "unable to get the linked InterceptorRoute for ScaledObject", "name", sor.Name, "namespace", sor.Namespace)
	return nil, err
}
