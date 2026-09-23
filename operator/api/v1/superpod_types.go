/*
Copyright 2026.

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

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// SuperpodSpec defines the desired state supplied by the user.
type SuperpodSpec struct {
	// superAbility is the text displayed on the nginx webpage.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=256
	// +required
	SuperAbility string `json:"superAbility"`

	// host is the DNS hostname used by the Ingress, without a scheme or path.
	// It must resolve to the cluster's Ingress controller to reach the webpage.
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?(\.[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)*$`
	// +required
	Host string `json:"host"`

	// ingressClassName selects the Ingress controller that serves the webpage.
	// When omitted, the cluster must handle classless Ingresses or provide a default IngressClass.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	// +optional
	IngressClassName string `json:"ingressClassName,omitempty"`
}

// SuperpodStatus records the state observed by the controller.
type SuperpodStatus struct {
	// podName is the name of the nginx Pod managed by this Superpod.
	// +optional
	PodName string `json:"podName,omitempty"`

	// url is the webpage address derived from the configured Ingress host.
	// An address alone does not guarantee that DNS and routing are ready.
	// +optional
	URL string `json:"url,omitempty"`

	// conditions describe the observed state. The Ready condition indicates
	// whether nginx is ready and the Ingress has an address. It does not verify
	// browser DNS, connectivity, or ConfigMap projection into the container.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="URL",type=string,JSONPath=".status.url"
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=".metadata.creationTimestamp"

// Superpod is the Schema for the superpods API
type Superpod struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of Superpod
	// +required
	Spec SuperpodSpec `json:"spec"`

	// status defines the observed state of Superpod
	// +optional
	Status SuperpodStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// SuperpodList contains a list of Superpod
type SuperpodList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []Superpod `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion, &Superpod{}, &SuperpodList{})
		return nil
	})
}
