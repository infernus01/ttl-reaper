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

// TTLReaperConfig defines a configuration for TTL-based cleanup of custom resources
type TTLReaperConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TTLReaperConfigSpec   `json:"spec,omitempty"`
	Status TTLReaperConfigStatus `json:"status,omitempty"`
}

// TTLReaperConfigSpec defines the desired state of TTLReaperConfig
type TTLReaperConfigSpec struct {
	// TargetNamespace is the namespace where the target resources exist
	// +optional
	TargetNamespace string `json:"targetNamespace,omitempty"`

	// TargetKind is the kind of custom resources to monitor for TTL cleanup
	TargetKind string `json:"targetKind"`

	// TargetAPIVersion is the API version of the target kind
	TargetAPIVersion string `json:"targetApiVersion"`

	// TTLFieldPath is the path to the TTL field in the target resource spec
	// Defaults to "spec.ttlSecondsAfterFinished"
	// +optional
	TTLFieldPath string `json:"ttlFieldPath,omitempty"`

	// CheckInterval defines how often to check for expired resources (in seconds)
	// Defaults to 300 seconds (5 minutes)
	// +optional
	CheckInterval *int32 `json:"checkInterval,omitempty"`

	// Enabled controls whether this configuration is active
	// +optional
	Enabled *bool `json:"enabled,omitempty"`
}

// TTLReaperConfigStatus defines the observed state of TTLReaperConfig
type TTLReaperConfigStatus struct {
	// LastProcessedTime is the last time resources were checked for TTL expiration
	// +optional
	LastProcessedTime *metav1.Time `json:"lastProcessedTime,omitempty"`

	// ProcessedCount is the number of resources processed in the last run
	// +optional
	ProcessedCount int32 `json:"processedCount,omitempty"`

	// DeletedCount is the number of resources deleted in the last run
	// +optional
	DeletedCount int32 `json:"deletedCount,omitempty"`

	// Conditions represents the latest available observations of the config's current state
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// TTLReaperConfigList contains a list of TTLReaperConfig
type TTLReaperConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TTLReaperConfig `json:"items"`
}
