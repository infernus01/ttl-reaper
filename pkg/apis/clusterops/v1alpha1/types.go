package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type TTLReaper struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TTLReaperSpec   `json:"spec,omitempty"`
	Status TTLReaperStatus `json:"status,omitempty"`
}

// TTLReaperSpec defines the desired state of TTLReaper
type TTLReaperSpec struct {
	// TargetKind specifies the kind of custom resource to monitor for TTL expiration
	TargetKind string `json:"targetKind"`

	// TargetAPIVersion specifies the API version of the target custom resource
	TargetAPIVersion string `json:"targetAPIVersion"`

	// TargetNamespace specifies the namespace to monitor. If empty, monitors all namespaces
	TargetNamespace string `json:"targetNamespace,omitempty"`

	// LabelSelector to filter which resources to monitor (optional)
	LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`
}

// TTLReaperStatus defines the observed state of TTLReaper
type TTLReaperStatus struct {
	// LastProcessedTime tracks when the reaper last processed resources
	LastProcessedTime *metav1.Time `json:"lastProcessedTime,omitempty"`

	// TotalReaped tracks total number of resources cleaned up
	TotalReaped int32 `json:"totalReaped,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// TTLReaperList contains a list of TTLReaper
type TTLReaperList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TTLReaper `json:"items"`
}
