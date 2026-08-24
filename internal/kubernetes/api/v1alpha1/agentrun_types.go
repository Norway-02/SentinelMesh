package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AgentRunPhase represents the lifecycle phase of an AgentRun.
type AgentRunPhase string

const (
	AgentRunPhasePending     AgentRunPhase = "Pending"
	AgentRunPhaseCreating    AgentRunPhase = "Creating"
	AgentRunPhaseRunning     AgentRunPhase = "Running"
	AgentRunPhaseFailed      AgentRunPhase = "Failed"
	AgentRunPhaseSucceeded   AgentRunPhase = "Succeeded"
	AgentRunPhaseFenced      AgentRunPhase = "Fenced"
	AgentRunPhaseQuarantined AgentRunPhase = "Quarantined"
	AgentRunPhaseUnknown     AgentRunPhase = "Unknown"
)

// AgentRunSpec defines the desired state of AgentRun.
type AgentRunSpec struct {
	// RunID is the SentinelMesh run identifier. Immutable.
	RunID string `json:"runID"`
	// AgentID is the SentinelMesh agent identifier.
	AgentID string `json:"agentID"`
	// ClusterID is the target cluster identifier.
	ClusterID string `json:"clusterID"`
	// NodeID is the scheduler-selected node name. The reconciler pins the Pod
	// to this node via spec.nodeName, bypassing the kube-scheduler.
	NodeID string `json:"nodeID"`
	// Image is the container image for the agent workload.
	Image string `json:"image"`
	// Resources defines the resource requirements for the agent container.
	Resources AgentRunResources `json:"resources"`
	// SecurityClass is an optional label for policy enforcement.
	SecurityClass string `json:"securityClass,omitempty"`
	// RestoreCheckpointID specifies the checkpoint to restore state from on recovery.
	RestoreCheckpointID string `json:"restoreCheckpointID,omitempty"`
	// RestoreStep specifies the sequence number / step to resume execution from.
	RestoreStep int64 `json:"restoreStep,omitempty"`
	// RecoveryGeneration tracks the failover attempt number.
	RecoveryGeneration int `json:"recoveryGeneration,omitempty"`
	// ExecutionGeneration tracks the authoritative execution attempt.
	ExecutionGeneration int `json:"executionGeneration,omitempty"`
	// FencingToken is the unforgeable authority token issued by the control plane.
	FencingToken string `json:"fencingToken,omitempty"`
}

// AgentRunResources defines CPU and memory requirements for an agent container.
type AgentRunResources struct {
	CPU    string `json:"cpu"`
	Memory string `json:"memory"`
}

// AgentRunStatus defines the observed state of AgentRun.
type AgentRunStatus struct {
	// Phase is the current lifecycle phase of the AgentRun.
	Phase AgentRunPhase `json:"phase,omitempty"`

	// PodName is the name of the Pod created for this AgentRun.
	PodName string `json:"podName,omitempty"`

	// NodeName is the Kubernetes node the Pod was scheduled to.
	NodeName string `json:"nodeName,omitempty"`

	// ExecutionGeneration is the generation observed during reconciliation.
	ExecutionGeneration int `json:"executionGeneration,omitempty"`

	// FencingToken is the token confirmed active for this run.
	FencingToken string `json:"fencingToken,omitempty"`

	// StartTime records when the Pod transitioned to Running.
	StartTime *metav1.Time `json:"startTime,omitempty"`

	// Message is a human-readable status message (e.g. error reason).
	Message string `json:"message,omitempty"`

	// Conditions represent detailed status conditions for this AgentRun.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Cluster",type="string",JSONPath=".spec.clusterID"
//+kubebuilder:printcolumn:name="Node",type="string",JSONPath=".spec.nodeID"
//+kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
//+kubebuilder:printcolumn:name="Gen",type="integer",JSONPath=".spec.executionGeneration"
//+kubebuilder:printcolumn:name="Pod",type="string",JSONPath=".status.podName"
//+kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// AgentRun is the Schema for the agentruns API.
type AgentRun struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AgentRunSpec   `json:"spec,omitempty"`
	Status AgentRunStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// AgentRunList contains a list of AgentRun
type AgentRunList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AgentRun `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AgentRun{}, &AgentRunList{})
}
