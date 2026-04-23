/*
Copyright 2026 The KEDA Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PathMatch defines a path matching rule.
type PathMatch struct {
	// Path prefix to match against. The longest matching prefix wins.
	// +kubebuilder:validation:MinLength=1
	Value string `json:"value"`
}

// HeaderMatch defines a header matching rule.
type HeaderMatch struct {
	// Name of the HTTP header.
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
	// Value to match against (exact match). If omitted, matches any value for given header name.
	// +optional
	Value *string `json:"value,omitzero"`
}

// RoutingRule defines a set of matching criteria for routing requests.
type RoutingRule struct {
	// Match any of these hostnames. Wildcard patterns (e.g. "*.example.com")
	// are supported. A single "*" acts as a catch-all. Exact matches take
	// priority over wildcards; more specific wildcards (e.g. "*.bar.example.com")
	// take priority over less specific ones.
	// +optional
	// +listType=set
	Hosts []string `json:"hosts,omitzero"`
	// Match any of these path prefixes. When multiple paths match,
	// the longest prefix wins.
	// +optional
	// +listType=atomic
	Paths []PathMatch `json:"paths,omitzero"`
	// All listed headers must match the request (AND semantics).
	// If a header's Value is omitted, the header must be present but any
	// value is accepted.
	// +optional
	// +listType=map
	// +listMapKey=name
	Headers []HeaderMatch `json:"headers,omitzero"`
}

// ConcurrencyTargetSpec defines concurrency-based scaling.
type ConcurrencyTargetSpec struct {
	// Target concurrent request count per replica.
	// +kubebuilder:validation:Minimum=1
	TargetValue int32 `json:"targetValue"`
}

// RequestRateTargetSpec defines rate-based scaling.
type RequestRateTargetSpec struct {
	// Bucket size for rate calculation within the window.
	// +kubebuilder:default="1s"
	// +optional
	Granularity metav1.Duration `json:"granularity,omitzero"`
	// Target request rate per replica.
	// +kubebuilder:validation:Minimum=1
	TargetValue int32 `json:"targetValue"`
	// Sliding time window over which the request rate is calculated.
	// +kubebuilder:default="1m"
	// +optional
	Window metav1.Duration `json:"window,omitzero"`
}

// ScalingMetricSpec defines what metric drives autoscaling.
// At least one of concurrency or requestRate must be set. When both are set,
// both metrics are reported and KEDA scales based on whichever demands
// more replicas.
// +kubebuilder:validation:XValidation:rule="has(self.concurrency) || has(self.requestRate)",message="at least one of 'concurrency' or 'requestRate' must be set"
type ScalingMetricSpec struct {
	// Scale based on concurrent request count.
	// +optional
	Concurrency *ConcurrencyTargetSpec `json:"concurrency,omitzero"`
	// Scale based on request rate.
	// +optional
	RequestRate *RequestRateTargetSpec `json:"requestRate,omitzero"`
}

// TargetRef identifies a Service to route traffic to.
// Exactly one of port or portName must be set.
// +kubebuilder:validation:XValidation:rule="has(self.port) != has(self.portName)",message="exactly one of 'port' or 'portName' must be set"
type TargetRef struct {
	// Name of the Kubernetes Service.
	// +kubebuilder:validation:MinLength=1
	Service string `json:"service"`
	// Port number on the Service. Mutually exclusive with portName.
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	Port int32 `json:"port,omitzero"`
	// Named port on the Service. Mutually exclusive with port.
	// +optional
	// +kubebuilder:validation:MinLength=1
	PortName string `json:"portName,omitzero"`
}

// AsServiceRef converts the TargetRef to a ServiceRef.
func (t TargetRef) AsServiceRef() ServiceRef {
	return ServiceRef{
		Name:     t.Service,
		Port:     t.Port,
		PortName: t.PortName,
	}
}

// ServiceRef identifies a Kubernetes Service by name and port.
// Exactly one of port or portName must be set.
// +kubebuilder:validation:XValidation:rule="has(self.port) != has(self.portName)",message="exactly one of 'port' or 'portName' must be set"
type ServiceRef struct {
	// Name of the Kubernetes Service.
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
	// Port number on the Service. Mutually exclusive with portName.
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	Port int32 `json:"port,omitzero"`
	// Named port on the Service. Mutually exclusive with port.
	// +optional
	// +kubebuilder:validation:MinLength=1
	PortName string `json:"portName,omitzero"`
}

// ColdStartFallback configures the fallback target for cold-start scenarios.
type ColdStartFallback struct {
	// Kubernetes Service to use as the fallback target.
	Service *ServiceRef `json:"service,omitzero"`
}

// ColdStartSpec configures behavior while the target is not ready.
// +kubebuilder:validation:XValidation:rule="has(self.fallback)",message="'fallback' must be set"
type ColdStartSpec struct {
	// Fallback target to route to when the primary backend does not become
	// ready within the readiness timeout.
	Fallback *ColdStartFallback `json:"fallback,omitzero"`
}

// InterceptorRouteTimeouts configures per-route request handling timeouts.
// When a field is unset, the global interceptor timeout configuration
// (KEDA_HTTP_*_TIMEOUT env vars) is used.
type InterceptorRouteTimeouts struct {
	// Time to wait for the backend to become ready (e.g. scale-from-zero).
	// Unset: uses the global KEDA_HTTP_READINESS_TIMEOUT (default: disabled).
	// Set to "0s" to disable the dedicated readiness deadline so the full
	// request budget is available for cold starts. When a fallback service
	// is configured and this is "0s", a 30s default is applied.
	// +optional
	Readiness *metav1.Duration `json:"readiness,omitzero"`

	// Total time allowed for the entire request lifecycle.
	// Unset: uses the global KEDA_HTTP_REQUEST_TIMEOUT (default: disabled).
	// Set to "0s" to disable the request deadline.
	// +optional
	Request *metav1.Duration `json:"request,omitzero"`

	// Max time to wait for the response headers from the backend after the
	// request has been fully sent. Does not include cold-start wait time.
	// Unset: uses the global KEDA_HTTP_RESPONSE_HEADER_TIMEOUT (default: 300s).
	// Set to "0s" to disable the response header deadline.
	// +optional
	ResponseHeader *metav1.Duration `json:"responseHeader,omitzero"`
}

// InterceptorRouteSpec defines the desired state of InterceptorRoute.
type InterceptorRouteSpec struct {
	// Backend service to route traffic to.
	Target TargetRef `json:"target"`
	// Cold start behavior when scaling from zero.
	// +optional
	ColdStart *ColdStartSpec `json:"coldStart,omitzero"`
	// Timeout configuration for request handling.
	// +optional
	Timeouts InterceptorRouteTimeouts `json:"timeouts,omitzero"`
	// Routing rules that define how requests are matched to this target.
	// +optional
	// +listType=atomic
	Rules []RoutingRule `json:"rules,omitzero"`
	// Metric configuration for autoscaling.
	ScalingMetric ScalingMetricSpec `json:"scalingMetric"`
}

// InterceptorRouteStatus defines the observed state of InterceptorRoute.
type InterceptorRouteStatus struct {
	// Conditions of the InterceptorRoute.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitzero"`
}

// InterceptorRoute configures request routing and autoscaling for a target service.
//
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="TargetService",type="string",JSONPath=".spec.target.service"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].status"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type InterceptorRoute struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitzero"`

	Spec InterceptorRouteSpec `json:"spec,omitzero"`
	// +optional
	Status InterceptorRouteStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// InterceptorRouteList contains a list of InterceptorRoute.
type InterceptorRouteList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []InterceptorRoute `json:"items"`
}

func init() {
	SchemeBuilder.Register(&InterceptorRoute{}, &InterceptorRouteList{})
}
