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
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	httpv1beta1 "github.com/kedacore/http-add-on/operator/apis/http/v1beta1"
	"github.com/kedacore/http-add-on/pkg/k8s"
	"github.com/kedacore/http-add-on/pkg/util"
)

var errNoMetricValues = errors.New("no metric values in response")

// interceptorMetricName names the metric that scales the interceptor fleet.
var interceptorMetricName = metricName("interceptor", "concurrency")

type scalerHandler struct {
	lggr           logr.Logger
	pinger         *queuePinger
	reader         client.Reader
	streamInterval time.Duration
	// specChanges wakes the StreamMetricSpec watchers whenever a resource that
	// defines scaling metrics may have changed.
	specChanges util.KeyedBroadcaster[types.NamespacedName]
	externalscaler.UnimplementedExternalScalerServer
}

func newScalerHandler(lggr logr.Logger, pinger *queuePinger, reader client.Reader, streamInterval time.Duration) *scalerHandler {
	return &scalerHandler{
		lggr:           lggr,
		pinger:         pinger,
		reader:         reader,
		streamInterval: streamInterval,
		specChanges:    util.NewKeyedBroadcaster[types.NamespacedName](),
	}
}

// notifySpecChange tells the open StreamMetricSpec streams to re-evaluate their
// metric specs. It is called from the informers watching the resources that
// carry the scaling metrics.
func (e *scalerHandler) notifySpecChange(key types.NamespacedName) {
	e.specChanges.Broadcast(key)
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
	// KEDA uses push events to activate targets, while standard metric polling
	// handles inactivation and scale-down.
	previousActive := false
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
			newlyActive := active.Result && !previousActive
			previousActive = active.Result
			if !newlyActive {
				continue
			}
			if err := server.Send(active); err != nil {
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

	if target, ok := scalerMetadata[k8s.InterceptorTargetPendingRequestsKey]; ok {
		return e.interceptorMetricSpec(target)
	}

	err := fmt.Errorf("unable to get InterceptorRoute reference")
	lggr.Error(err, "unable to get the linked InterceptorRoute for ScaledObject", "name", sor.Name, "namespace", sor.Namespace)
	return nil, err
}

func (e *scalerHandler) interceptorMetricSpec(interceptorTargetPendingRequests string) (*externalscaler.GetMetricSpecResponse, error) {
	targetPendingRequests, err := strconv.ParseFloat(interceptorTargetPendingRequests, 64)
	if err != nil {
		e.lggr.Error(err, "unable to parse the interceptor target", "key", k8s.InterceptorTargetPendingRequestsKey, "value", interceptorTargetPendingRequests)
		return nil, err
	}

	// KEDA turns any other value into a target of 0, which the HPA rejects. It
	// rounds targets to three decimals, so 0.001 is the smallest usable one.
	if math.IsNaN(targetPendingRequests) || math.IsInf(targetPendingRequests, 0) || targetPendingRequests < 0.001 {
		err := fmt.Errorf("%s must be a finite number of at least 0.001, got %q", k8s.InterceptorTargetPendingRequestsKey, interceptorTargetPendingRequests)
		e.lggr.Error(err, "invalid interceptor target", "key", k8s.InterceptorTargetPendingRequestsKey, "value", interceptorTargetPendingRequests)
		return nil, err
	}

	return &externalscaler.GetMetricSpecResponse{
		MetricSpecs: []*externalscaler.MetricSpec{
			{
				MetricName:      interceptorMetricName,
				TargetSizeFloat: targetPendingRequests,
			},
		},
	}, nil
}

// StreamMetricSpec pushes the metric specs to KEDA every time they change.
// The target values live in the InterceptorRoute instead of the ScaledObject,
// so without this stream KEDA would keep the targets it read when it built its
// scaler cache and the HPA would never pick up an edited target value.
func (e *scalerHandler) StreamMetricSpec(sor *externalscaler.ScaledObjectRef, server externalscaler.ExternalScaler_StreamMetricSpecServer) error {
	lggr := e.lggr.WithName("StreamMetricSpec")
	ctx := server.Context()

	irName := sor.GetScalerMetadata()[k8s.InterceptorRouteKey]
	key := types.NamespacedName{Namespace: sor.Namespace, Name: irName}
	changed, unsubscribe := e.specChanges.Subscribe(key)
	defer unsubscribe()

	var sent *externalscaler.GetMetricSpecResponse
	for {
		res, err := e.GetMetricSpec(ctx, sor)
		if err != nil {
			lggr.Error(err, "unable to get metric spec", "scaledObjectRef", sor.String())
			return err
		}

		// KEDA expects the current specs right after the stream is opened, and
		// every send makes it rebuild the HPA, so afterwards we only send when
		// the targets actually changed.
		if !proto.Equal(res, sent) {
			if err := server.Send(res); err != nil {
				lggr.Error(err, "unable to send metric spec in stream", "scaledObjectRef", sor.String())
				return err
			}
			sent = res
		}

		select {
		case <-ctx.Done():
			return nil
		case <-changed:
		}
	}
}

func (e *scalerHandler) GetMetrics(ctx context.Context, metricRequest *externalscaler.GetMetricsRequest) (*externalscaler.GetMetricsResponse, error) {
	lggr := e.lggr.WithName("GetMetrics")
	sor := metricRequest.ScaledObjectRef
	if sor == nil {
		return nil, status.Error(codes.InvalidArgument, "scaledObjectRef is required")
	}

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

		// KEDA sets metricName per metric; empty means return all (used by IsActive).
		requestedMetric := metricRequest.GetMetricName()

		var metricValues []*externalscaler.MetricValue
		if ir.Spec.ScalingMetric.Concurrency != nil && (requestedMetric == "" || requestedMetric == ConcurrencyMetricName(irName)) {
			metricValues = append(metricValues, &externalscaler.MetricValue{
				MetricName:       ConcurrencyMetricName(irName),
				MetricValueFloat: float64(count.Concurrency),
			})
		}
		if ir.Spec.ScalingMetric.RequestRate != nil && (requestedMetric == "" || requestedMetric == RateMetricName(irName)) {
			metricValues = append(metricValues, &externalscaler.MetricValue{
				MetricName:       RateMetricName(irName),
				MetricValueFloat: count.RequestRate,
			})
		}

		if requestedMetric != "" && len(metricValues) == 0 {
			lggr.V(1).Info("requested metric not found", "metricName", requestedMetric, "interceptorRouteName", irName)
		}

		return &externalscaler.GetMetricsResponse{
			MetricValues: metricValues,
		}, nil
	}

	if _, ok := scalerMetadata[k8s.InterceptorTargetPendingRequestsKey]; ok {
		if requested := metricRequest.GetMetricName(); requested != "" && requested != interceptorMetricName {
			return &externalscaler.GetMetricsResponse{}, nil
		}
		return e.interceptorMetrics(), nil
	}

	err := fmt.Errorf("unable to get InterceptorRoute reference")
	lggr.Error(err, "unable to get the linked InterceptorRoute for ScaledObject", "name", sor.Name, "namespace", sor.Namespace)
	return nil, err
}

// interceptorMetrics reports the pending requests summed over all routes, which
// is the load on the interceptor fleet.
func (e *scalerHandler) interceptorMetrics() *externalscaler.GetMetricsResponse {
	var count float64
	for _, c := range e.pinger.counts() {
		count += float64(c.Concurrency)
	}

	return &externalscaler.GetMetricsResponse{
		MetricValues: []*externalscaler.MetricValue{
			{
				MetricName:       interceptorMetricName,
				MetricValueFloat: count,
			},
		},
	}
}
