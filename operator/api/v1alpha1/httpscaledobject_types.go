/*


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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// HTTPScaledObjectCreationStatus describes the creation status
// of the scaler's additional resources such as Services, Ingresses and Deployments
// +kubebuilder:validation:Enum=Created;Error;Pending;Unknown;Terminating;Terminated;Ready
type HTTPScaledObjectCreationStatus string

// HTTPScaledObjectConditionReason describes the reason why the condition transitioned
// +kubebuilder:validation:Enum=ErrorCreatingExternalScaler;ErrorCreatingExternalScalerService;CreatedExternalScaler;ErrorCreatingInterceptorScaledObject;ErrorCreatingAppScaledObject;AppScaledObjectCreated;InterceptorScaledObjectCreated;ErrorCreatingInterceptor;ErrorCreatingInterceptorAdminService;ErrorCreatingInterceptorProxyService;InterceptorCreated;TerminatingResources;InterceptorDeploymentTerminated;InterceptorDeploymentTerminationError;InterceptorAdminServiceTerminationError;InterceptorAdminServiceTerminated;InterceptorProxyServiceTerminationError;InterceptorProxyServiceTerminated;ExternalScalerDeploymentTerminationError;ExternalScalerDeploymentTerminated;ExternalScalerServiceTerminationError;ExternalScalerServiceTerminated;InterceptorScaledObjectTerminated;AppScaledObjectTerminated;AppScaledObjectTerminationError;InterceptorScaledObjectTerminationError;PendingCreation;HTTPScaledObjectIsReady;
type HTTPScaledObjectConditionReason string

const (
	ErrorCreatingExternalScaler              HTTPScaledObjectConditionReason = "ErrorCreatingExternalScaler"
	ErrorCreatingExternalScalerService       HTTPScaledObjectConditionReason = "ErrorCreatingExternalScalerService"
	CreatedExternalScaler                    HTTPScaledObjectConditionReason = "CreatedExternalScaler"
	ErrorCreatingInterceptorScaledObject     HTTPScaledObjectConditionReason = "ErrorCreatingInterceptorScaledObject"
	ErrorCreatingAppScaledObject             HTTPScaledObjectConditionReason = "ErrorCreatingAppScaledObject"
	AppScaledObjectCreated                   HTTPScaledObjectConditionReason = "AppScaledObjectCreated"
	InterceptorScaledObjectCreated           HTTPScaledObjectConditionReason = "InterceptorScaledObjectCreated"
	ErrorCreatingInterceptor                 HTTPScaledObjectConditionReason = "ErrorCreatingInterceptor"
	ErrorCreatingInterceptorAdminService     HTTPScaledObjectConditionReason = "ErrorCreatingInterceptorAdminService"
	ErrorCreatingInterceptorProxyService     HTTPScaledObjectConditionReason = "ErrorCreatingInterceptorProxyService"
	InterceptorCreated                       HTTPScaledObjectConditionReason = "InterceptorCreated"
	TerminatingResources                     HTTPScaledObjectConditionReason = "TerminatingResources"
	InterceptorDeploymentTerminated          HTTPScaledObjectConditionReason = "InterceptorDeploymentTerminated"
	InterceptorDeploymentTerminationError    HTTPScaledObjectConditionReason = "InterceptorDeploymentTerminationError"
	InterceptorAdminServiceTerminationError  HTTPScaledObjectConditionReason = "InterceptorAdminServiceTerminationError"
	InterceptorAdminServiceTerminated        HTTPScaledObjectConditionReason = "InterceptorAdminServiceTerminated"
	InterceptorProxyServiceTerminationError  HTTPScaledObjectConditionReason = "InterceptorProxyServiceTerminationError"
	InterceptorProxyServiceTerminated        HTTPScaledObjectConditionReason = "InterceptorProxyServiceTerminated"
	ExternalScalerDeploymentTerminationError HTTPScaledObjectConditionReason = "ExternalScalerDeploymentTerminationError"
	ExternalScalerDeploymentTerminated       HTTPScaledObjectConditionReason = "ExternalScalerDeploymentTerminated"
	ExternalScalerServiceTerminationError    HTTPScaledObjectConditionReason = "ExternalScalerServiceTerminationError"
	ExternalScalerServiceTerminated          HTTPScaledObjectConditionReason = "ExternalScalerServiceTerminated"
	InterceptorScaledObjectTerminated        HTTPScaledObjectConditionReason = "InterceptorScaledObjectTerminated"
	AppScaledObjectTerminated                HTTPScaledObjectConditionReason = "AppScaledObjectTerminated"
	AppScaledObjectTerminationError          HTTPScaledObjectConditionReason = "AppScaledObjectTerminationError"
	InterceptorScaledObjectTerminationError  HTTPScaledObjectConditionReason = "InterceptorScaledObjectTerminationError"
	PendingCreation                          HTTPScaledObjectConditionReason = "PendingCreation"
	HTTPScaledObjectIsReady                  HTTPScaledObjectConditionReason = "HTTPScaledObjectIsReady"
)

const (
	// Created indicates the resource has been created
	Created HTTPScaledObjectCreationStatus = "Created"
	// Terminated indicates the resource has been terminated
	Terminated HTTPScaledObjectCreationStatus = "Terminated"
	// Error indicates the resource had an error
	Error HTTPScaledObjectCreationStatus = "Error"
	// Pending indicates the resource hasn't been created
	Pending HTTPScaledObjectCreationStatus = "Pending"
	// Terminating indicates that the resource is marked for deletion but hasn't
	// been deleted yet
	Terminating HTTPScaledObjectCreationStatus = "Terminating"
	// Unknown indicates the status is unavailable
	Unknown HTTPScaledObjectCreationStatus = "Unknown"
	// Ready indicates the object is fully created
	Ready HTTPScaledObjectCreationStatus = "Ready"
)

// Condition to store the condition state
type HTTPScaledObjectCondition struct {
	// Timestamp of the condition
	// +optional
	Timestamp string `json:"timestamp" description:"Timestamp of this condition"`
	// Type of condition
	// +required
	Type HTTPScaledObjectCreationStatus `json:"type" description:"type of status condition"`
	// Status of the condition, one of True, False, Unknown.
	// +required
	Status metav1.ConditionStatus `json:"status" description:"status of the condition, one of True, False, Unknown"`
	// The reason for the condition's last transition.
	// +optional
	Reason HTTPScaledObjectConditionReason `json:"reason,omitempty" description:"one-word CamelCase reason for the condition's last transition"`
	// A human readable message indicating details about the transition.
	// +optional
	Message string `json:"message,omitempty" description:"human-readable message indicating details about last transition"`
}

// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.
// Important: Run "make" to regenerate code after modifying this file

type ReplicaStruct struct {
	// Minimum amount of replicas to have in the deployment (Default 0)
	Min int32 `json:"min,omitempty" description:"Minimum amount of replicas to have in the deployment (Default 0)"`
	// Maximum amount of replicas to have in the deployment (Default 100)
	Max int32 `json:"max,omitempty" description:"Maximum amount of replicas to have in the deployment (Default 100)"`
}

type HTTPScaledObjectServiceSpec struct {
	Type corev1.ServiceType
	ServicePort int32
}

// HTTPScaledObjectSpec defines the desired state of HTTPScaledObject
type HTTPScaledObjectSpec struct {
	// The name of the deployment to route HTTP requests to (and to autoscale). Either this
	// or Image must be set
	ScaleTargetRef *ScaleTargetRef `json:"scaleTargetRef"`
	// (optional) Replica information
	//+optional
	Replicas ReplicaStruct `json:"replicas,omitempty"`
	Service HTTPScaledObjectServiceSpec `json:"service,omitempty"`
}

// ScaleTargetRef contains all the details about an HTTP application to scale and route to
type ScaleTargetRef struct {
	// The name of the deployment to scale according to HTTP traffic
	Deployment string `json:"deployment"`
	// The name of the service to route to
	Service string `json:"service"`
	// The port to route to
	Port int32 `json:"port"`
}

// HTTPScaledObjectStatus defines the observed state of HTTPScaledObject
type HTTPScaledObjectStatus struct {
	// List of auditable conditions of the operator
	Conditions []HTTPScaledObjectCondition `json:"conditions,omitempty" description:"List of auditable conditions of the operator"`
}

// +kubebuilder:object:root=true

// HTTPScaledObject is the Schema for the scaledobjects API
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
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
