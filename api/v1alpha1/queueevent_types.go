package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

type QueueEventSpec struct {
	NodeName string `json:"nodeName,omitempty"`
	Reason   string `json:"reason,omitempty"`
	Severity string `json:"severity,omitempty"`
	Message  string `json:"message,omitempty"`
	Status   string `json:"status,omitempty"`
}

type QueueEventStatus struct {
	ProcessedAt metav1.Time `json:"processedAt,omitempty"`
	Message     string      `json:"message,omitempty"`
}

type QueueEvent struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   QueueEventSpec   `json:"spec,omitempty"`
	Status QueueEventStatus `json:"status,omitempty"`
}

type QueueEventList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []QueueEvent `json:"items"`
}
