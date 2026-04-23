// Handlers contains the gRPC implementation for an external scaler as defined
// by the KEDA documentation at https://keda.sh/docs/2.0/concepts/external-scalers/#built-in-scalers-interface
// This is the interface KEDA will poll in order to get the request queue size
// and scale user apps properly
package main

import (
	"context"
	"errors"
	"fmt"
	"math"
	"slices"
	"strconv"
	"time"

	"github.com/go-logr/logr"
	"github.com/kedacore/keda/v2/pkg/scalers/externalscaler"
	"google.golang.org/protobuf/types/known/emptypb"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	httpv1alpha1 "github.com/kedacore/http-add-on/operator/apis/http/v1alpha1"
	httpv1beta1 "github.com/kedacore/http-add-on/operator/apis/http/v1beta1"
	"github.com/kedacore/http-add-on/pkg/k8s"
)

const (
	keyInterceptorTargetPendingRequests = "interceptorTargetPendingRequests"
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

	// TODO(v1): remove the following when deprecating HTTPScaledObject
	httpScaledObjectName, ok := scalerMetadata[k8s.HTTPScaledObjectKey]
	if !ok {
		if scalerMetadata != nil {
			if interceptorTargetPendingRequests, ok := scalerMetadata[keyInterceptorTargetPendingRequests]; ok {
				// generated the metric name for the ScaledObject targeting the interceptor
				metricName := MetricNameHTTPSO(k8s.NamespacedNameFromScaledObjectRef(sor))
				return e.interceptorMetricSpec(metricName, interceptorTargetPendingRequests)
			}
		}
		err := fmt.Errorf("unable to get HTTPScaledObject reference")
		lggr.Error(err, "unable to get the linked HTTPScaledObject for ScaledObject", "name", sor.Name, "namespace", sor.Namespace, "httpScaledObjectName", httpScaledObjectName)
		return nil, err
	}

	httpso := &httpv1alpha1.HTTPScaledObject{}
	if err := e.reader.Get(ctx, types.NamespacedName{Namespace: sor.Namespace, Name: httpScaledObjectName}, httpso); err != nil {
		lggr.Error(err, "unable to get HTTPScaledObject", "name", sor.Name, "namespace", sor.Namespace, "httpScaledObjectName", httpScaledObjectName)
		return nil, err
	}

	// generated the metric name for HTTPScaledObject
	metricName := MetricNameHTTPSO(k8s.NamespacedNameFromNameAndNamespace(httpScaledObjectName, sor.Namespace))

	targetValue := int64(ptr.Deref(httpso.Spec.TargetPendingRequests, 100))

	if httpso.Spec.ScalingMetric != nil {
		if httpso.Spec.ScalingMetric.Concurrency != nil {
			targetValue = int64(httpso.Spec.ScalingMetric.Concurrency.TargetValue)
		}
		if httpso.Spec.ScalingMetric.Rate != nil {
			targetValue = int64(httpso.Spec.ScalingMetric.Rate.TargetValue)
		}
	}

	res := &externalscaler.GetMetricSpecResponse{
		MetricSpecs: []*externalscaler.MetricSpec{
			{
				MetricName: metricName,
				TargetSize: targetValue,
			},
		},
	}
	return res, nil
}

func (e *scalerHandler) interceptorMetricSpec(metricName string, interceptorTargetPendingRequests string) (*externalscaler.GetMetricSpecResponse, error) {
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

	// TODO(v1): remove the following when deprecating HTTPScaledObject
	httpScaledObjectName, ok := scalerMetadata[k8s.HTTPScaledObjectKey]
	if !ok {
		if scalerMetadata != nil {
			if _, ok := scalerMetadata[keyInterceptorTargetPendingRequests]; ok {
				// generated the metric name for the ScaledObject targeting the interceptor
				metricName := MetricNameHTTPSO(k8s.NamespacedNameFromScaledObjectRef(sor))
				return e.interceptorMetrics(metricName)
			}
		}
		err := fmt.Errorf("unable to get HTTPScaledObject reference")
		lggr.Error(err, "unable to get the linked HTTPScaledObject for ScaledObject", "name", sor.Name, "namespace", sor.Namespace, "httpScaledObjectName", httpScaledObjectName)
		return nil, err
	}

	httpso := &httpv1alpha1.HTTPScaledObject{}
	if err := e.reader.Get(ctx, types.NamespacedName{Namespace: sor.Namespace, Name: httpScaledObjectName}, httpso); err != nil {
		lggr.Error(err, "unable to get HTTPScaledObject", "name", httpScaledObjectName, "namespace", sor.Namespace, "httpScaledObjectName", httpScaledObjectName)
		return nil, err
	}

	// generated the metric name for HTTPScaledObject
	namespacedName := k8s.NamespacedNameFromNameAndNamespace(httpScaledObjectName, sor.Namespace)
	metricName := MetricNameHTTPSO(namespacedName)

	key := namespacedName.String()

	if httpso.Spec.ScalingMetric != nil && httpso.Spec.ScalingMetric.Rate != nil {
		e.pinger.UpdateBucketConfig(key, httpso.Spec.ScalingMetric.Rate.Window.Duration, httpso.Spec.ScalingMetric.Rate.Granularity.Duration)
	}

	count := e.pinger.count(key)

	var metricValue int
	if httpso.Spec.ScalingMetric != nil && httpso.Spec.ScalingMetric.Rate != nil {
		metricValue = int(math.Ceil(count.RequestRate))
		lggr.V(1).Info(fmt.Sprintf("%d rps for %s", metricValue, httpso.GetName()))
	} else {
		metricValue = count.Concurrency
		lggr.V(1).Info(fmt.Sprintf("%d concurrent requests for %s", metricValue, httpso.GetName()))
	}

	res := &externalscaler.GetMetricsResponse{
		MetricValues: []*externalscaler.MetricValue{
			{
				MetricName:  metricName,
				MetricValue: int64(metricValue),
			},
		},
	}
	return res, nil
}

func (e *scalerHandler) interceptorMetrics(metricName string) (*externalscaler.GetMetricsResponse, error) {
	lggr := e.lggr.WithName("interceptorMetrics")

	var count int64
	for _, v := range e.pinger.counts() {
		count += int64(v.Concurrency)
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
