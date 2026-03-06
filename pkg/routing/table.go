package routing

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	httpv1alpha1 "github.com/kedacore/http-add-on/operator/apis/http/v1alpha1"
	httpv1beta1 "github.com/kedacore/http-add-on/operator/apis/http/v1beta1"
	"github.com/kedacore/http-add-on/pkg/k8s"
	"github.com/kedacore/http-add-on/pkg/queue"
	"github.com/kedacore/http-add-on/pkg/util"
)

var errNotSyncedTable = errors.New("table has not synced")

type Table interface {
	util.HealthChecker

	HasSynced() bool
	Route(req *http.Request) *httpv1beta1.InterceptorRoute
	Signal()
	Start(ctx context.Context) error
}

type table struct {
	memoryHolder   util.AtomicValue[*TableMemory]
	memorySignaler util.Signaler
	previousKeys   map[string]struct{}
	reader         client.Reader
	queueCounter   queue.Counter
}

var _ Table = (*table)(nil)

func NewTable(reader client.Reader, counter queue.Counter) Table {
	return &table{
		memorySignaler: util.NewSignaler(),
		previousKeys:   make(map[string]struct{}),
		queueCounter:   counter,
		reader:         reader,
	}
}

func (t *table) refreshMemory(ctx context.Context) error {
	for {
		var irList httpv1beta1.InterceptorRouteList
		if err := t.reader.List(ctx, &irList); err != nil {
			return fmt.Errorf("failed to list InterceptorRoutes: %w", err)
		}

		tm := NewTableMemory()
		currentKeys := make(map[string]struct{})

		for i := range irList.Items {
			ir := &irList.Items[i]
			key := k8s.ResourceKey(ir.Namespace, ir.Name)

			currentKeys[key] = struct{}{}

			tm = tm.Remember(ir)

			window := time.Minute
			granularity := time.Second
			if ir.Spec.ScalingMetric.RequestRate != nil {
				window = ir.Spec.ScalingMetric.RequestRate.Window.Duration
				granularity = ir.Spec.ScalingMetric.RequestRate.Granularity.Duration
			}

			t.queueCounter.UpdateBuckets(key, window, granularity)
		}

		// TODO(v1): remove the HTTPSO to IR conversion
		var httpsoList httpv1alpha1.HTTPScaledObjectList
		if err := t.reader.List(ctx, &httpsoList); err != nil {
			return fmt.Errorf("listing HTTPScaledObjects: %w", err)
		}

		for i := range httpsoList.Items {
			httpso := &httpsoList.Items[i]
			key := fmt.Sprintf("%s/%s", httpso.Namespace, httpso.Name)

			if _, ok := currentKeys[key]; ok {
				// skip the conflicting HTTPSO, IR takes precedence
				continue
			}
			currentKeys[key] = struct{}{}

			// Create an IR from the HTTPSO to simplify the whole routing logic
			ir := &httpv1beta1.InterceptorRoute{
				ObjectMeta: metav1.ObjectMeta{
					CreationTimestamp: httpso.CreationTimestamp,
					Name:              httpso.Name,
					Namespace:         httpso.Namespace,
				},
				Spec: httpv1beta1.InterceptorRouteSpec{
					Target: httpv1beta1.TargetRef{
						Port:     httpso.Spec.ScaleTargetRef.Port,
						PortName: httpso.Spec.ScaleTargetRef.PortName,
						Service:  httpso.Spec.ScaleTargetRef.Service,
					},
				},
			}

			rr := httpv1beta1.RoutingRule{
				Hosts: httpso.Spec.Hosts,
			}
			for _, pathPrefix := range httpso.Spec.PathPrefixes {
				rr.Paths = append(rr.Paths, httpv1beta1.PathMatch{
					Value: pathPrefix,
				})
			}
			for _, header := range httpso.Spec.Headers {
				rr.Headers = append(rr.Headers, httpv1beta1.HeaderMatch{
					Name:  header.Name,
					Value: header.Value,
				})
			}
			ir.Spec.Rules = []httpv1beta1.RoutingRule{rr}

			// Temporarily store the HTTPSO timeouts in an internal annotation of the IR to not expose them.
			if httpso.Spec.Timeouts != nil {
				if ir.Annotations == nil {
					ir.Annotations = make(map[string]string)
				}
				if httpso.Spec.Timeouts.ConditionWait.Duration > 0 {
					ir.Annotations[k8s.AnnotationConditionWaitTimeout] = httpso.Spec.Timeouts.ConditionWait.Duration.String()
				}
				if httpso.Spec.Timeouts.ResponseHeader.Duration > 0 {
					ir.Annotations[k8s.AnnotationResponseHeaderTimeout] = httpso.Spec.Timeouts.ResponseHeader.Duration.String()
				}
			}

			if c := httpso.Spec.ColdStartTimeoutFailoverRef; c != nil {
				ir.Spec.ColdStart = &httpv1beta1.ColdStartSpec{
					Fallback: &httpv1beta1.TargetRef{
						Port:     c.Port,
						PortName: c.PortName,
						Service:  c.Service,
					},
				}
				if c.TimeoutSeconds > 0 {
					if ir.Annotations == nil {
						ir.Annotations = make(map[string]string)
					}
					ir.Annotations[k8s.AnnotationConditionWaitTimeout] = (time.Duration(c.TimeoutSeconds) * time.Second).String()
				}
			}

			if httpso.Spec.ScalingMetric != nil {
				if httpso.Spec.ScalingMetric.Concurrency != nil {
					ir.Spec.ScalingMetric.Concurrency = &httpv1beta1.ConcurrencyTargetSpec{
						TargetValue: int32(httpso.Spec.ScalingMetric.Concurrency.TargetValue), //nolint:gosec // kubebuilder-validated field, overflow not possible
					}
				}
				if httpso.Spec.ScalingMetric.Rate != nil {
					ir.Spec.ScalingMetric.RequestRate = &httpv1beta1.RequestRateTargetSpec{
						TargetValue: int32(httpso.Spec.ScalingMetric.Rate.TargetValue), //nolint:gosec // kubebuilder-validated field, overflow not possible
						Window:      httpso.Spec.ScalingMetric.Rate.Window,
						Granularity: httpso.Spec.ScalingMetric.Rate.Granularity,
					}
				}
			}

			tm = tm.Remember(ir)

			// Ensure queue counter bucket exists
			window := time.Minute
			granularity := time.Second
			if httpso.Spec.ScalingMetric != nil && httpso.Spec.ScalingMetric.Rate != nil {
				window = httpso.Spec.ScalingMetric.Rate.Window.Duration
				granularity = httpso.Spec.ScalingMetric.Rate.Granularity.Duration
			}
			t.queueCounter.UpdateBuckets(key, window, granularity)
		}

		for key := range t.previousKeys {
			if _, exists := currentKeys[key]; !exists {
				t.queueCounter.RemoveKey(key)
			}
		}
		t.previousKeys = currentKeys

		t.memoryHolder.Set(tm)

		if err := t.memorySignaler.Wait(ctx); err != nil {
			return err
		}
	}
}

func (t *table) Signal() {
	t.memorySignaler.Signal()
}

func (t *table) Start(ctx context.Context) error {
	return t.refreshMemory(ctx)
}

func (t *table) Route(req *http.Request) *httpv1beta1.InterceptorRoute {
	if req == nil || req.URL == nil {
		return nil
	}

	tm := t.memoryHolder.Get()
	if tm == nil {
		return nil
	}

	hostname := stripPort(req.Host)

	return tm.Route(hostname, req.URL.Path, req.Header)
}

func (t *table) HasSynced() bool {
	tm := t.memoryHolder.Get()
	return tm != nil
}

var _ util.HealthChecker = (*table)(nil)

func (t *table) HealthCheck(_ context.Context) error {
	// TODO: HasSynced never fails after passing once, it is not testing health over time
	if !t.HasSynced() {
		return errNotSyncedTable
	}

	return nil
}
