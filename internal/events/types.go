package events

import (
	"encoding/json"
	"time"
)

// Event wrapper structure as defined in Stage 6 requirements
type Event struct {
	EventID       string          `json:"event_id"`
	EventType     string          `json:"event_type"`
	SchemaVersion int             `json:"schema_version"`
	AggregateType string          `json:"aggregate_type"`
	AggregateID   string          `json:"aggregate_id"`
	TenantID      string          `json:"tenant_id"`
	CorrelationID string          `json:"correlation_id"`
	TraceParent   string          `json:"traceparent,omitempty"`
	OccurredAt    time.Time       `json:"occurred_at"`
	Payload       json.RawMessage `json:"payload"`
}

// Payload structs for the domain events

type AgentCreatedPayload struct {
	AgentID   string    `json:"agent_id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

type AgentDeletedPayload struct {
	AgentID   string    `json:"agent_id"`
	DeletedAt time.Time `json:"deleted_at"`
}

type RunCreatedPayload struct {
	RunID     string    `json:"run_id"`
	AgentID   string    `json:"agent_id"`
	CreatedAt time.Time `json:"created_at"`
}

type RunStateChangedPayload struct {
	RunID     string `json:"run_id"`
	AgentID   string `json:"agent_id"`
	FromState string `json:"from_state"`
	ToState   string `json:"to_state"`
	Version   int64  `json:"version"`
}

type CheckpointMetadataPayload struct {
	ID        string `json:"id"`
	Sequence  int64  `json:"sequence"`
	Checksum  string `json:"checksum"`
	SizeBytes int64  `json:"size_bytes,omitempty"`
}

type RunScheduledPayload struct {
	RunID               string                     `json:"run_id"`
	AgentID             string                     `json:"agent_id"`
	ClusterID           string                     `json:"cluster_id"`
	NodeID              string                     `json:"node_id"`
	AlgorithmVersion    string                     `json:"algorithm_version"`
	ExecutionGeneration int                        `json:"execution_generation"`
	FencingToken        string                     `json:"fencing_token"`
	FinalScore          float64                    `json:"final_score"`
	Scores              map[string]float64         `json:"scores"`
	AgentImage          string                     `json:"agent_image"`
	AgentCPU            string                     `json:"agent_cpu"`
	AgentMemory         string                     `json:"agent_memory"`
	Checkpoint          *CheckpointMetadataPayload `json:"checkpoint,omitempty"`
}

type RunSchedulingFailedPayload struct {
	RunID       string `json:"run_id"`
	AgentID     string `json:"agent_id"`
	Reason      string `json:"reason"`
	IsTransient bool   `json:"is_transient"`
}

type RunExecutionFencedPayload struct {
	RunID                   string    `json:"run_id"`
	AgentID                 string    `json:"agent_id"`
	ClusterID               string    `json:"cluster_id"`
	PodName                 string    `json:"pod_name"`
	StaleGeneration         int       `json:"stale_generation"`
	AuthoritativeGeneration int       `json:"authoritative_generation"`
	FencedToken             string    `json:"fenced_token"`
	Reason                  string    `json:"reason"`
	FencedAt                time.Time `json:"fenced_at"`
}

type SecurityViolationPayload struct {
	EventID       string    `json:"event_id"`
	RunID         string    `json:"run_id"`
	AgentID       string    `json:"agent_id"`
	TenantID      string    `json:"tenant_id"`
	CorrelationID string    `json:"correlation_id"`
	Source        string    `json:"source"`
	Operation     string    `json:"operation"`
	Resource      string    `json:"resource"`
	RuleID        string    `json:"rule_id"`
	Decision      string    `json:"decision"`
	Severity      string    `json:"severity"`
	Reason        string    `json:"reason"`
	OccurredAt    time.Time `json:"occurred_at"`
}

type CheckpointSavedPayload struct {
	CheckpointID   string    `json:"checkpoint_id"`
	RunID          string    `json:"run_id"`
	AgentID        string    `json:"agent_id"`
	TenantID       string    `json:"tenant_id"`
	SequenceNumber int64     `json:"sequence_number"`
	StateChecksum  string    `json:"state_checksum"`
	SizeBytes      int64     `json:"size_bytes"`
	CreatedAt      time.Time `json:"created_at"`
}

type ClusterNodeFailedPayload struct {
	ClusterID      string    `json:"cluster_id"`
	NodeID         string    `json:"node_id"`
	Reason         string    `json:"reason"`
	FailedAt       time.Time `json:"failed_at"`
	AffectedRunIDs []string  `json:"affected_run_ids,omitempty"`
}

type ClusterUnreachablePayload struct {
	ClusterID      string    `json:"cluster_id"`
	Region         string    `json:"region"`
	Reason         string    `json:"reason"`
	UnreachableAt  time.Time `json:"unreachable_at"`
	AffectedRunIDs []string  `json:"affected_run_ids,omitempty"`
}

type ClusterHeartbeatPayload struct {
	ClusterID       string    `json:"cluster_id"`
	Health          string    `json:"health"`
	TotalCPU        float64   `json:"total_cpu"`
	TotalMemory     float64   `json:"total_memory"`
	AvailableCPU    float64   `json:"available_cpu"`
	AvailableMemory float64   `json:"available_memory"`
	Timestamp       time.Time `json:"timestamp"`
}

type RunRecoveryRequestedPayload struct {
	RunID              string    `json:"run_id"`
	AgentID            string    `json:"agent_id"`
	TenantID           string    `json:"tenant_id"`
	FailedClusterID    string    `json:"failed_cluster_id,omitempty"`
	FailedNodeID       string    `json:"failed_node_id,omitempty"`
	RecoveryGeneration int       `json:"recovery_generation"`
	SourceCheckpointID string    `json:"source_checkpoint_id,omitempty"`
	SequenceNumber     int64     `json:"sequence_number,omitempty"`
	RequestedAt        time.Time `json:"requested_at"`
}

type RunRecoveredPayload struct {
	RunID                string    `json:"run_id"`
	AgentID              string    `json:"agent_id"`
	TenantID             string    `json:"tenant_id"`
	TargetClusterID      string    `json:"target_cluster_id"`
	TargetNodeID         string    `json:"target_node_id"`
	RecoveryGeneration   int       `json:"recovery_generation"`
	FencingToken         string    `json:"fencing_token"`
	RestoredCheckpointID string    `json:"restored_checkpoint_id"`
	RestoredSequence     int64     `json:"restored_sequence"`
	RecoveredAt          time.Time `json:"recovered_at"`
}

type RunRecoveryFailedPayload struct {
	RunID              string    `json:"run_id"`
	AgentID            string    `json:"agent_id"`
	TenantID           string    `json:"tenant_id"`
	RecoveryGeneration int       `json:"recovery_generation"`
	Reason             string    `json:"reason"`
	FailedAt           time.Time `json:"failed_at"`
}

type RunVerificationRequestedPayload struct {
	RunID       string    `json:"run_id"`
	AgentID     string    `json:"agent_id"`
	TenantID    string    `json:"tenant_id"`
	RequestedAt time.Time `json:"requested_at"`
}

type RunVerifiedPayload struct {
	RunID            string    `json:"run_id"`
	AgentID          string    `json:"agent_id"`
	TenantID         string    `json:"tenant_id"`
	AttestationID    string    `json:"attestation_id"`
	EvidenceDigest   string    `json:"evidence_digest"`
	RulesPassedCount int       `json:"rules_passed_count"`
	VerifiedAt       time.Time `json:"verified_at"`
}

type RunVerificationFailedPayload struct {
	RunID          string    `json:"run_id"`
	AgentID        string    `json:"agent_id"`
	TenantID       string    `json:"tenant_id"`
	AttestationID  string    `json:"attestation_id"`
	EvidenceDigest string    `json:"evidence_digest"`
	FailedRuleID   string    `json:"failed_rule_id"`
	Reason         string    `json:"reason"`
	FailedAt       time.Time `json:"failed_at"`
}

type ModelRoutingDecidedPayload struct {
	TaskID             string             `json:"task_id"`
	RunID              string             `json:"run_id"`
	AgentID            string             `json:"agent_id"`
	TenantID           string             `json:"tenant_id"`
	SelectedModelID    string             `json:"selected_model_id"`
	SelectedTier       string             `json:"selected_tier"`
	Policy             string             `json:"policy"`
	EstimatedCostUSD   float64            `json:"estimated_cost_usd"`
	EstimatedLatencyMs float64            `json:"estimated_latency_ms"`
	PredictedQuality   float64            `json:"predicted_quality"`
	FallbackCandidates []string           `json:"fallback_candidates"`
	ScoreBreakdown     map[string]float64 `json:"score_breakdown"`
	DecidedAt          time.Time          `json:"decided_at"`
}

type ModelInvocationCompletedPayload struct {
	TaskID         string    `json:"task_id"`
	RunID          string    `json:"run_id"`
	ModelID        string    `json:"model_id"`
	ActualDuration time.Duration `json:"actual_duration"`
	ActualCostUSD  float64   `json:"actual_cost_usd"`
	PromptTokens   int       `json:"prompt_tokens"`
	OutputTokens   int       `json:"output_tokens"`
	QualityScore   float64   `json:"quality_score"`
	FallbackUsed   bool      `json:"fallback_used"`
	CompletedAt    time.Time `json:"completed_at"`
}

type ModelInvocationFailedPayload struct {
	TaskID    string    `json:"task_id"`
	RunID     string    `json:"run_id"`
	ModelID   string    `json:"model_id"`
	ErrorCode string    `json:"error_code"`
	Reason    string    `json:"reason"`
	FailedAt  time.Time `json:"failed_at"`
}

type ModelFallbackTriggeredPayload struct {
	TaskID         string    `json:"task_id"`
	RunID          string    `json:"run_id"`
	FailedModelID  string    `json:"failed_model_id"`
	FallbackModelID string   `json:"fallback_model_id"`
	AttemptNumber  int       `json:"attempt_number"`
	Reason         string    `json:"reason"`
	TriggeredAt    time.Time `json:"triggered_at"`
}

type AdaptiveRoutingDecidedPayload struct {
	TaskID              string             `json:"task_id"`
	RunID               string             `json:"run_id"`
	AgentID             string             `json:"agent_id"`
	TenantID            string             `json:"tenant_id"`
	SelectedModelID     string             `json:"selected_model_id"`
	SelectedTier        string             `json:"selected_tier"`
	Policy              string             `json:"policy"`
	LearningModelVersion string            `json:"learning_model_version"`
	FeatureSchemaVersion string            `json:"feature_schema_version"`
	PriorVersion        string             `json:"prior_version"`
	DriftDetectorVersion string            `json:"drift_detector_version"`
	Confidence          float64            `json:"confidence"`
	SampleCount         int                `json:"sample_count"`
	PredictedSuccess    float64            `json:"predicted_success"`
	SuccessLowerCI      float64            `json:"success_lower_ci"`
	SuccessUpperCI      float64            `json:"success_upper_ci"`
	PredictedQuality    float64            `json:"predicted_quality"`
	QualityVariance     float64            `json:"quality_variance"`
	PredictedLatencyMs  float64            `json:"predicted_latency_ms"`
	PredictedP95LatMs   float64            `json:"predicted_p95_lat_ms"`
	PredictedCostUSD    float64            `json:"predicted_cost_usd"`
	EffectiveUtility    float64            `json:"effective_utility"`
	FallbackCandidates  []string           `json:"fallback_candidates"`
	ScoreBreakdown      map[string]float64 `json:"score_breakdown"`
	DecidedAt           time.Time          `json:"decided_at"`
}

type ModelPerformanceDriftPayload struct {
	ModelID          string    `json:"model_id"`
	Metric           string    `json:"metric"` // "quality_drop", "latency_spike", "failure_spike"
	BaselineValue    float64   `json:"baseline_value"`
	RecentValue      float64   `json:"recent_value"`
	DeltaPercentage  float64   `json:"delta_percentage"`
	Threshold        float64   `json:"threshold"`
	ActionTaken      string    `json:"action_taken"` // "marked_degraded", "penalized_utility"
	DetectedAt       time.Time `json:"detected_at"`
}

type OnlinePolicyDecidedPayload struct {
	TaskID             string             `json:"task_id"`
	RunID              string             `json:"run_id"`
	AgentID            string             `json:"agent_id"`
	TenantID           string             `json:"tenant_id"`
	SelectedModelID    string             `json:"selected_model_id"`
	DecisionMode       string             `json:"decision_mode"` // "EXPLOIT", "EXPLORE", "CANARY", "SHADOW"
	PolicyVersion      string             `json:"policy_version"`
	RewardVersion      string             `json:"reward_version"`
	ExplorationVersion string             `json:"exploration_version"`
	ExpectedUtility    float64            `json:"expected_utility"`
	UCBScore           float64            `json:"ucb_score"`
	Uncertainty        float64            `json:"uncertainty"`
	ExplorationRate    float64            `json:"exploration_rate"`
	FallbackCandidates []string           `json:"fallback_candidates"`
	ScoreBreakdown     map[string]float64 `json:"score_breakdown"`
	DecidedAt          time.Time          `json:"decided_at"`
}

type PolicyRollbackPayload struct {
	CurrentVersion   string    `json:"current_version"`
	TargetVersion    string    `json:"target_version"`
	TriggerMetric    string    `json:"trigger_metric"`
	ObservedValue    float64   `json:"observed_value"`
	ThresholdValue   float64   `json:"threshold_value"`
	Reason           string    `json:"reason"`
	RolledBackAt     time.Time `json:"rolled_back_at"`
}

type ShadowPolicyEvaluatedPayload struct {
	TaskID              string    `json:"task_id"`
	LiveModelID         string    `json:"live_model_id"`
	ShadowModelID       string    `json:"shadow_model_id"`
	LiveExpectedReward  float64   `json:"live_expected_reward"`
	ShadowExpectedReward float64  `json:"shadow_expected_reward"`
	Agreement           bool      `json:"agreement"`
	EvaluatedAt         time.Time `json:"evaluated_at"`
}
