/*
Copyright 2022.

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
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// EntanglementSpec defines the desired state of Entanglement
type EntanglementSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	ServiceUUID string  `json:"serviceUUID,omitempty"`
	ServiceRef  *string `json:"serviceRef,omitempty"`
	SecretRef   *string `json:"secretRef,omitempty"`
	Host        string  `json:"host,omitempty"`
	Port        string  `json:"port,omitempty"`
	HostNetwork bool    `json:"hostNetwork,omitempty"`
	Inbound     bool    `json:"inbound,omitempty"`
	// +kubebuilder:validation:Optional
	ServiceSpec *v1.ServiceSpec `json:"serviceSpec,omitEmpty"`
}

// EntanglementStatus defines the observed state of Entanglement
type EntanglementStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Entanglement is the Schema for the entanglements API
type Entanglement struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   EntanglementSpec   `json:"spec,omitempty"`
	Status EntanglementStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// EntanglementList contains a list of Entanglement
type EntanglementList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Entanglement `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Entanglement{}, &EntanglementList{})
}
