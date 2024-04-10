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

// ScaleTargetRef contains all the details about an HTTP application to scale and route to
type ScaleTargetRef struct {
	// Deprecated: The name of the deployment to scale according to HTTP traffic
	// +optional
	Deployment string `json:"deployment"`
	// +optional
	Name string `json:"name"`
	// +optional
	APIVersion string `json:"apiVersion,omitempty"`
	// +optional
	Kind string `json:"kind,omitempty"`
	// The name of the service to route to
	Service string `json:"service"`
	// The port to route to
	Port int32 `json:"port"`
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
	// The name of the deployment to route HTTP requests to (and to autoscale).
	ScaleTargetRef ScaleTargetRef `json:"scaleTargetRef"`
	// (optional) Replica information
	// +optional
	Replicas *ReplicaStruct `json:"replicas,omitempty"`
	// (optional) DEPRECATED (use SscalingMetric instead) Target metric value
	// +optional
	TargetPendingRequests *int32 `json:"targetPendingRequests,omitempty" description:"The target metric value for the HPA (Default 100)"`
	// (optional) Cooldown period value
	// +optional
	CooldownPeriod *int32 `json:"scaledownPeriod,omitempty" description:"Cooldown period (seconds) for resources to scale down (Default 300)"`
	// (optional) Configuration for the metric used for scaling
	// +optional
	ScalingMetric *ScalingMetricSpec `json:"scalingMetric,omitempty" description:"Configuration for the metric used for scaling. If empty 'concurrency' will be used"`
}

// HTTPScaledObjectStatus defines the observed state of HTTPScaledObject
type HTTPScaledObjectStatus struct {
	// TargetWorkload reflects details about the scaled workload.
	// +optional
	TargetWorkload string `json:"targetWorkload,omitempty" description:"It reflects details about the scaled workload"`
	// TargetService reflects details about the scaled service.
	// +optional
	TargetService string `json:"targetService,omitempty" description:"It reflects details about the scaled service"`
	// Conditions of the operator
	Conditions Conditions `json:"conditions,omitempty" description:"List of auditable conditions of the operator"`
}

// +genclient
// +k8s:openapi-gen=true
// +kubebuilder:object:root=true
// +kubebuilder:printcolumn:name="TargetWorkload",type="string",JSONPath=".status.targetWorkload"
// +kubebuilder:printcolumn:name="TargetService",type="string",JSONPath=".status.targetService"
// +kubebuilder:printcolumn:name="MinReplicas",type="integer",JSONPath=".spec.replicas.min"
// +kubebuilder:printcolumn:name="MaxReplicas",type="integer",JSONPath=".spec.replicas.max"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="Active",type="string",JSONPath=".status.conditions[?(@.type==\"HTTPScaledObjectIsReady\")].status"
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
