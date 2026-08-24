package types

import (
	"time"
)

// AgentState represents the lifecycle state of an AgentRun
type AgentState string

const (
	StateCreated       AgentState = "CREATED"
	StateQueued        AgentState = "QUEUED"
	StateScheduled     AgentState = "SCHEDULED"
	StateStarting      AgentState = "STARTING"
	StateRunning       AgentState = "RUNNING"
	StatePaused        AgentState = "PAUSED"
	StateCheckpointing AgentState = "CHECKPOINTING"
	StateVerifying     AgentState = "VERIFYING"
	StateCompleted     AgentState = "COMPLETED"
	StateFailed        AgentState = "FAILED"
	StateRecovering    AgentState = "RECOVERING"
	StateRestoring     AgentState = "RESTORING"
	StateCancelled     AgentState = "CANCELLED"
)

// IsTerminal returns true if the state is a final, non-recoverable state.
func (s AgentState) IsTerminal() bool {
	return s == StateCompleted || s == StateCancelled
}

// IsNonTerminal returns true if the state is part of the active execution or recovery lifecycle.
func (s AgentState) IsNonTerminal() bool {
	return !s.IsTerminal()
}

// Agent defines the desired state and requirements of an AI agent workload.
type Agent struct {
	ID                 string             `json:"id"`
	Name               string             `json:"name"`
	Tenant             string             `json:"tenant"`
	Version            string             `json:"version"`
	Image              string             `json:"image"`
	Resources          AgentResources     `json:"resources"`
	Priority           string             `json:"priority"`
	SecurityPolicy     SecurityPolicy     `json:"security_policy"`
	NetworkPolicy      NetworkPolicy      `json:"network_policy"`
	CheckpointPolicy   CheckpointPolicy   `json:"checkpoint_policy"`
	VerificationPolicy VerificationPolicy `json:"verification_policy"`
	Status             string             `json:"status"`
	CreatedAt          time.Time          `json:"created_at"`
	UpdatedAt          time.Time          `json:"updated_at"`
}

type AgentResources struct {
	CPU    string `json:"cpu"`
	Memory string `json:"memory"`
	GPU    int    `json:"gpu"`
}

type SecurityPolicy struct {
	Profile string `json:"profile"`
}

type NetworkPolicy struct {
	Mode string `json:"mode"`
}

type CheckpointPolicy struct {
	Enabled  bool   `json:"enabled"`
	Interval string `json:"interval"`
}

// ArtifactRule defines verification for output files.
type ArtifactRule struct {
	ID               string `json:"id"`
	Path             string `json:"path"`
	Required         bool   `json:"required"`
	MinSizeBytes     int64  `json:"min_size_bytes,omitempty"`
	ExpectedChecksum string `json:"expected_checksum,omitempty"` // Canonical SHA-256
	SchemaJSON       string `json:"schema_json,omitempty"`       // Required JSON fields
}

// KubernetesStateRule defines verification of Kubernetes infrastructure state.
type KubernetesStateRule struct {
	ID                string `json:"id"`
	Namespace         string `json:"namespace"`
	PodNamePrefix     string `json:"pod_name_prefix"`
	ExpectedPhase     string `json:"expected_phase"` // e.g. Running, Succeeded
	MinReadyReplicas  int32  `json:"min_ready_replicas,omitempty"`
	MaxRestarts       int32  `json:"max_restarts,omitempty"`
}

// HTTPHealthRule defines verification via HTTP endpoint probes.
type HTTPHealthRule struct {
	ID                    string `json:"id"`
	URL                   string `json:"url"`
	Method                string `json:"method,omitempty"` // GET, POST
	ExpectedStatus        int    `json:"expected_status"`  // 200
	ExpectedBodySubstring string `json:"expected_body_substring,omitempty"`
	TimeoutSeconds        int    `json:"timeout_seconds,omitempty"`
}

// InvariantRule defines assertion checks on metrics or key-value outputs.
type InvariantRule struct {
	ID            string `json:"id"`
	MetricName    string `json:"metric_name"`
	Operator      string `json:"operator"` // eq, neq, gt, gte, lt, lte, matches
	ExpectedValue string `json:"expected_value"`
}

// CommandRule defines verification via isolated script / test execution.
type CommandRule struct {
	ID               string   `json:"id"`
	Command          string   `json:"command"`
	Args             []string `json:"args,omitempty"`
	WorkingDir       string   `json:"working_dir,omitempty"`
	ExpectedExitCode int      `json:"expected_exit_code"`
	TimeoutSeconds   int      `json:"timeout_seconds,omitempty"`
}

// VerificationPolicy specifies outcome assertions that must pass before COMPLETED state is awarded.
type VerificationPolicy struct {
	Enabled         bool                  `json:"enabled"`
	ArtifactRules   []ArtifactRule        `json:"artifact_rules,omitempty"`
	KubernetesRules []KubernetesStateRule `json:"kubernetes_rules,omitempty"`
	HTTPRules       []HTTPHealthRule      `json:"http_rules,omitempty"`
	InvariantRules  []InvariantRule       `json:"invariant_rules,omitempty"`
	CommandRules    []CommandRule         `json:"command_rules,omitempty"`
}

// AgentRun represents a specific execution instance of an Agent.
type AgentRun struct {
	ID                string     `json:"id"`
	AgentID           string     `json:"agent_id"`
	State             AgentState `json:"state"`
	Node              string     `json:"node"`
	Cluster           string     `json:"cluster"`
	StartedAt         *time.Time `json:"started_at,omitempty"`
	FinishedAt        *time.Time `json:"finished_at,omitempty"`
	LastCheckpoint    string     `json:"last_checkpoint"`
	RetryCount        int        `json:"retry_count"`
	ResourceUsage     string     `json:"resource_usage"`
	CostEstimate      float64    `json:"cost_estimate"`
	VerificationState string     `json:"verification_state"`
	FailureReason     string     `json:"failure_reason"`
}

// Checkpoint represents a saved state of an AgentRun.
type Checkpoint struct {
	ID          string    `json:"id"`
	RunID       string    `json:"run_id"`
	Version     int       `json:"version"`
	State       string    `json:"state"`
	ArtifactURI string    `json:"artifact_uri"`
	CreatedAt   time.Time `json:"created_at"`
	Checksum    string    `json:"checksum"`
}

// Policy defines rules for an agent's execution.
type Policy struct {
	ID              string   `json:"id"`
	Tenant          string   `json:"tenant"`
	FilesystemRules []string `json:"filesystem_rules"`
	NetworkRules    []string `json:"network_rules"`
	ToolRules       []string `json:"tool_rules"`
	ResourceRules   []string `json:"resource_rules"`
	SecretRules     []string `json:"secret_rules"`
	ApprovalRules   []string `json:"approval_rules"`
}

// VerificationResult holds the deterministic proof of an agent's success/failure.
type VerificationResult struct {
	RunID     string    `json:"run_id"`
	Checks    []string  `json:"checks"`
	Passed    bool      `json:"passed"`
	Failed    []string  `json:"failed"`
	Evidence  string    `json:"evidence"`
	Timestamp time.Time `json:"timestamp"`
}
