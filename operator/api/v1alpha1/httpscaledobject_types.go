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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// HTTPScaledObjectCreationStatus describes the creation status
// of the scaler's additional resources such as Services, Ingresses and Deployments
// +kubebuilder:validation:Enum=Created;Error;Pending;Unknown;Terminating
type HTTPScaledObjectCreationStatus string

const (
	// Created indicates the resource has been created
	Created HTTPScaledObjectCreationStatus = "Created"
	// Deleted indicates the resource has been deleted
	Deleted HTTPScaledObjectCreationStatus = "Created"
	// Error indicates the resource had an error
	Error HTTPScaledObjectCreationStatus = "Error"
	// Pending indicates the resource hasn't been created
	Pending HTTPScaledObjectCreationStatus = "Pending"
	// Terminating indicates that the resource is marked for deletion but hasn't
	// been deleted yet
	Terminating HTTPScaledObjectCreationStatus = "Terminating"
	// Unknown indicates the status is unavailable
	Unknown HTTPScaledObjectCreationStatus = "Unknown"
)

// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.
// Important: Run "make" to regenerate code after modifying this file

// HTTPScaledObjectSpec defines the desired state of HTTPScaledObject
type HTTPScaledObjectSpec struct {
	// (optional) The name of the application to be created.
	AppName string `json:"app_name,omitempty"`
	// The image this application will use.
	Image string `json:"app_image"`
	// The port this application will serve on.
	Port int32 `json:"port"`
	// (optional) The interval to check for changes.
	PollingInterval int32 `json:"polling_interval,omitempty"`
}
// TODO: Add ingress configurations

// HTTPScaledObjectStatus defines the observed state of HTTPScaledObject
type HTTPScaledObjectStatus struct {
	ServiceStatus      HTTPScaledObjectCreationStatus `json:"service_status,omitempty"`
	DeploymentStatus   HTTPScaledObjectCreationStatus `json:"deployment_status,omitempty"`
	ScaledObjectStatus HTTPScaledObjectCreationStatus `json:"scaledobject_status,omitempty"`
	InterceptorStatus HTTPScaledObjectCreationStatus `json:"interceptor_status,omitempty"`
	ExternalScalerStatus HTTPScaledObjectCreationStatus `json:"externalscaler_status,omitempty"`
	Ready              bool                           `json:"ready,omitempty"`
}

// +kubebuilder:object:root=true

// HTTPScaledObject is the Schema for the scaledobjects API
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
