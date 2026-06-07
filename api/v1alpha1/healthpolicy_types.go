package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

type HealthPolicySpec struct {
	CheckIntervalSeconds       int32      `json:"checkIntervalSeconds,omitempty"`
	RemediationCooldownSeconds int32      `json:"remediationCooldownSeconds,omitempty"`
	PrometheusURL              string     `json:"prometheusUrl,omitempty"`
	TargetNamespaces           []string   `json:"targetNamespaces,omitempty"`
	PodRules                   []PodRule  `json:"podRules,omitempty"`
	NodeRules                  []NodeRule `json:"nodeRules,omitempty"`
}

type PodRule struct {
	Name   string `json:"name,omitempty"`
	Query  string `json:"query,omitempty"`
	Action string `json:"action,omitempty"`
}

type NodeRule struct {
	Name      string `json:"name,omitempty"`
	Query     string `json:"query,omitempty"`
	Action    string `json:"action,omitempty"`
	Threshold string `json:"threshold,omitempty"`
}

type HealthPolicyStatus struct {
	LastCheckTime metav1.Time `json:"lastCheckTime,omitempty"`
	FindingsCount int32       `json:"findingsCount,omitempty"`
	LastMessage   string      `json:"lastMessage,omitempty"`
}

type HealthPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HealthPolicySpec   `json:"spec,omitempty"`
	Status HealthPolicyStatus `json:"status,omitempty"`
}

type HealthPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []HealthPolicy `json:"items"`
}
