/*
Copyright 2024 The TTL Reaper Authors.

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

// +k8s:deepcopy-gen=package
// +groupName=ttlreaper.io

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type TTLReaperConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TTLReaperConfigSpec   `json:"spec,omitempty"`
	Status TTLReaperConfigStatus `json:"status,omitempty"`
}

type TTLReaperConfigSpec struct {
	//the namespace where the target resources exist
	TargetNamespace string `json:"targetNamespace,omitempty"`

	// the kind of custom resources to monitor for TTL cleanup
	TargetKind string `json:"targetKind"`

	// the API version of the target kind
	TargetAPIVersion string `json:"targetApiVersion, omitempty"`

	// the path to the TTL field in the target resource spec
	TTLFieldPath string `json:"ttlFieldPath,omitempty"` 

	// how often to check for expired resources (in seconds)
	CheckInterval *int32 `json:"checkInterval,omitempty"`
}

type TTLReaperConfigStatus struct {
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type TTLReaperConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TTLReaperConfig `json:"items"`
}
