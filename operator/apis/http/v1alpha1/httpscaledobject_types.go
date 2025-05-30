/*
Copyright 2023 The KEDA Authors.

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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:object:generate=false
type Ref interface {
	GetServiceName() string
	GetPort() int32
	GetPortName() string
}

// ScaleTargetRef contains all the details about an HTTP application to scale and route to
type ScaleTargetRef struct {
	// +optional
	Name string `json:"name"`
	// +optional
	APIVersion string `json:"apiVersion,omitempty"`
	// +optional
	Kind string `json:"kind,omitempty"`
	// The name of the service to route to
	Service string `json:"service"`
	// The port to route to
	Port int32 `json:"port,omitempty"`
	// The port to route to referenced by name
	PortName string `json:"portName,omitempty"`
}

func (s ScaleTargetRef) GetServiceName() string {
	return s.Service
}

func (s ScaleTargetRef) GetPort() int32 {
	return s.Port
}

func (s ScaleTargetRef) GetPortName() string {
	return s.PortName
}

// ColdStartTimeoutFailoverRef contains all the details about an HTTP application to scale and route to
type ColdStartTimeoutFailoverRef struct {
	// The name of the service to route to
	Service string `json:"service"`
	// The port to route to
	Port int32 `json:"port,omitempty"`
	// The port to route to referenced by name
	PortName string `json:"portName,omitempty"`
	// The timeout in seconds to wait before routing to the failover service (Default 30)
	// +kubebuilder:default=30
	// +optional
	TimeoutSeconds int32 `json:"timeoutSeconds,omitempty"`
}

func (s *ColdStartTimeoutFailoverRef) GetServiceName() string {
	return s.Service
}

func (s *ColdStartTimeoutFailoverRef) GetPort() int32 {
	return s.Port
}

func (s *ColdStartTimeoutFailoverRef) GetPortName() string {
	return s.PortName
}

// ReplicaStruct contains the minimum and maximum amount of replicas to have in the deployment
type ReplicaStruct struct {
	// Minimum amount of replicas to have in the deployment (Default 0)
	Min *int32 `json:"min,omitempty" description:"Minimum amount of replicas to have in the deployment (Default 0)"`
	// Maximum amount of replicas to have in the deployment (Default 100)
	Max *int32 `json:"max,omitempty" description:"Maximum amount of replicas to have in the deployment (Default 100)"`
}

// ScalingMetricSpec contains the scaling calculation type
type ScalingMetricSpec struct {
	// Scaling based on concurrent requests for a given target
	Concurrency *ConcurrencyMetricSpec `json:"concurrency,omitempty" description:"Scaling based on concurrent requests for a given target. 'concurrency' and 'rate' are mutually exclusive."`
	// Scaling based the average rate during an specific time window for a given target
	Rate *RateMetricSpec `json:"requestRate,omitempty" description:"Scaling based the average rate during an specific time window for a given target. 'concurrency' and 'rate' are mutually exclusive."`
}

// ConcurrencyMetricSpec defines the concurrency scaling
type ConcurrencyMetricSpec struct {
	// Target value for rate scaling
	// +kubebuilder:default=100
	// +optional
	TargetValue int `json:"targetValue" description:"Target value for concurrency scaling"`
}

// RateMetricSpec defines the concurrency scaling
type RateMetricSpec struct {
	// Target value for rate scaling
	// +kubebuilder:default=100
	// +optional
	TargetValue int `json:"targetValue" description:"Target value for rate scaling"`
	// Time window for rate calculation
	// +kubebuilder:default="1m"
	// +optional
	Window metav1.Duration `json:"window" description:"Time window for rate calculation"`
	// Time granularity for rate calculation
	// +kubebuilder:default="1s"
	// +optional
	Granularity metav1.Duration `json:"granularity" description:"Time granularity for rate calculation"`
}

// HTTPScaledObjectTimeoutsSpec defines timeouts that override the global ones
type HTTPScaledObjectTimeoutsSpec struct {
	// How long to wait for the backing workload to have 1 or more replicas before connecting and sending the HTTP request (Default is set by the KEDA_CONDITION_WAIT_TIMEOUT environment variable)
	// +optional
	ConditionWait metav1.Duration `json:"conditionWait" description:"How long to wait for the backing workload to have 1 or more replicas before connecting and sending the HTTP request"`

	// How long to wait between when the HTTP request is sent to the backing app and when response headers need to arrive (Default is set by the KEDA_RESPONSE_HEADER_TIMEOUT environment variable)
	// +optional
	ResponseHeader metav1.Duration `json:"responseHeader" description:"How long to wait between when the HTTP request is sent to the backing app and when response headers need to arrive"`
}

// Header contains the definition for matching on header name and/or value
type Header struct {
	// +kubebuilder:validation:MinLength=1
	Name  string  `json:"name"`
	Value *string `json:"value,omitempty"`
}

// PlaceholderConfig defines the configuration for serving placeholder pages during scale-from-zero
type PlaceholderConfig struct {
	// Enable placeholder page when replicas are scaled to zero
	// +kubebuilder:default=false
	// +optional
	Enabled bool `json:"enabled" description:"Enable placeholder page when replicas are scaled to zero"`
	// Inline content for placeholder page (can be HTML, JSON, plain text, etc.)
	// +optional
	Content string `json:"content,omitempty" description:"Inline content for placeholder page"`
	// HTTP status code to return with placeholder page
	// +kubebuilder:default=503
	// +kubebuilder:validation:Minimum=100
	// +kubebuilder:validation:Maximum=599
	// +optional
	StatusCode int32 `json:"statusCode,omitempty" description:"HTTP status code to return with placeholder page (Default 503)"`
	// Refresh interval for client-side polling in seconds
	// +kubebuilder:default=5
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=60
	// +optional
	RefreshInterval int32 `json:"refreshInterval,omitempty" description:"Refresh interval for client-side polling in seconds (Default 5)"`
	// Additional HTTP headers to include with placeholder response
	// +optional
	Headers map[string]string `json:"headers,omitempty" description:"Additional HTTP headers to include with placeholder response"`
}

// HTTPScaledObjectSpec defines the desired state of HTTPScaledObject
type HTTPScaledObjectSpec struct {
	// The hosts to route. All requests which the "Host" header
	// matches any .spec.hosts (and the Request Target matches any
	// .spec.pathPrefixes) will be routed to the Service and Port specified in
	// the scaleTargetRef.
	Hosts []string `json:"hosts,omitempty"`
	// The paths to route. All requests which the Request Target matches any
	// .spec.pathPrefixes (and the "Host" header matches any .spec.hosts)
	// will be routed to the Service and Port specified in
	// the scaleTargetRef.
	// +optional
	PathPrefixes []string `json:"pathPrefixes,omitempty"`
	// The custom headers used to route. Once Hosts and PathPrefixes have been matched,
	// it will look for requests which have headers that match all the headers defined
	// in .spec.headers, it will be routed to the Service and Port specified in
	// the scaleTargetRef. Interceptor will take precedence for most specific header match.
	// If the headers can't be matched, then use first one without .spec.headers supplied
	// If that doesn't exist then routing will fail.
	// +optional
	// +listType=map
	// +listMapKey=name
	Headers []Header `json:"headers,omitempty"`
	// The name of the deployment to route HTTP requests to (and to autoscale).
	// Including validation as a requirement to define either the PortName or the Port
	// +kubebuilder:validation:XValidation:rule="has(self.portName) != has(self.port)",message="must define either the 'portName' or the 'port'"
	ScaleTargetRef ScaleTargetRef `json:"scaleTargetRef"`
	// (optional) The name of the failover service to route HTTP requests to when the target is not available
	// +optional
	// +kubebuilder:validation:XValidation:rule="has(self.portName) != has(self.port)",message="must define either the 'portName' or the 'port'"
	ColdStartTimeoutFailoverRef *ColdStartTimeoutFailoverRef `json:"coldStartTimeoutFailoverRef,omitempty"`
	// (optional) Replica information
	// +optional
	Replicas *ReplicaStruct `json:"replicas,omitempty"`
	// (optional) DEPRECATED (use ScalingMetric instead) Target metric value
	// +optional
	TargetPendingRequests *int32 `json:"targetPendingRequests,omitempty" description:"The target metric value for the HPA (Default 100)"`
	// (optional) Cooldown period value
	// +optional
	CooldownPeriod *int32 `json:"scaledownPeriod,omitempty" description:"Cooldown period (seconds) for resources to scale down (Default 300)"`
	// (optional) Initial period before scaling
	// +optional
	InitialCooldownPeriod *int32 `json:"initialCooldownPeriod,omitempty" description:"Initial period (seconds) before scaling (Default 0)"`
	// (optional) Configuration for the metric used for scaling
	// +optional
	ScalingMetric *ScalingMetricSpec `json:"scalingMetric,omitempty" description:"Configuration for the metric used for scaling. If empty 'concurrency' will be used"`
	// (optional) Timeouts that override the global ones
	// +optional
	Timeouts *HTTPScaledObjectTimeoutsSpec `json:"timeouts,omitempty" description:"Timeouts that override the global ones"`
	// (optional) Configuration for placeholder pages during scale-from-zero
	// +optional
	PlaceholderConfig *PlaceholderConfig `json:"placeholderConfig,omitempty" description:"Configuration for placeholder pages during scale-from-zero"`
}

// HTTPScaledObjectStatus defines the observed state of HTTPScaledObject
type HTTPScaledObjectStatus struct {
	// TargetWorkload reflects details about the scaled workload.
	// +optional
	TargetWorkload string `json:"targetWorkload,omitempty" description:"It reflects details about the scaled workload"`
	// TargetService reflects details about the scaled service.
	// +optional
	TargetService string `json:"targetService,omitempty" description:"It reflects details about the scaled service"`
	// Conditions of the HTTPScaledObject.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty" description:"Represents the current state of the HTTPScaledObject"`
}

// +genclient
// +k8s:openapi-gen=true
// +kubebuilder:object:root=true
// +kubebuilder:printcolumn:name="TargetWorkload",type="string",JSONPath=".status.targetWorkload"
// +kubebuilder:printcolumn:name="TargetService",type="string",JSONPath=".status.targetService"
// +kubebuilder:printcolumn:name="MinReplicas",type="integer",JSONPath=".spec.replicas.min"
// +kubebuilder:printcolumn:name="MaxReplicas",type="integer",JSONPath=".spec.replicas.max"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].status"
// +kubebuilder:resource:shortName=httpso
// +kubebuilder:subresource:status

// HTTPScaledObject is the Schema for the httpscaledobjects API
type HTTPScaledObject struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HTTPScaledObjectSpec   `json:"spec,omitempty"`
	Status HTTPScaledObjectStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// HTTPScaledObjectList contains a list of HTTPScaledObject
type HTTPScaledObjectList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HTTPScaledObject `json:"items"`
}

func init() {
	SchemeBuilder.Register(&HTTPScaledObject{}, &HTTPScaledObjectList{})
}
