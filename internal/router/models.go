package router

import (
	"time"
)

// TaskComplexity defines the explicit computational/reasoning level of a task.
type TaskComplexity string

const (
	ComplexitySimple          TaskComplexity = "simple"
	ComplexityModerate        TaskComplexity = "moderate"
	ComplexityComplex         TaskComplexity = "complex"
	ComplexityReasoningHeavy  TaskComplexity = "reasoning_heavy"
)

// ModelTier categorizes model scale.
type ModelTier string

const (
	TierSmall  ModelTier = "small"
	TierMedium ModelTier = "medium"
	TierLarge  ModelTier = "large"
)

// RoutingPolicy defines the optimization objective for model selection.
type RoutingPolicy string

const (
	PolicyStatic           RoutingPolicy = "static"
	PolicyCostOptimized    RoutingPolicy = "cost_optimized"
	PolicyLatencyOptimized RoutingPolicy = "latency_optimized"
	PolicyQualityOptimized RoutingPolicy = "quality_optimized"
	PolicyBalanced         RoutingPolicy = "balanced"
)

// ModelHealthStatus reflects the operational availability of a model endpoint.
type ModelHealthStatus string

const (
	HealthHealthy     ModelHealthStatus = "HEALTHY"
	HealthDegraded    ModelHealthStatus = "DEGRADED"
	HealthUnavailable ModelHealthStatus = "UNAVAILABLE"
)

// ModelDefinition represents a registered model endpoint and its capability metadata.
type ModelDefinition struct {
	ID                   string                             `json:"id"`
	Name                 string                             `json:"name"`
	Tier                 ModelTier                          `json:"tier"`
	Provider             string                             `json:"provider"`
	ProviderModelID      string                             `json:"provider_model_id,omitempty"`
	CostPer1kInputTokens  float64                            `json:"cost_per_1k_input_tokens"`
	CostPer1kOutputTokens float64                            `json:"cost_per_1k_output_tokens"`
	NominalP50LatencyMs  float64                            `json:"nominal_p50_latency_ms"`
	NominalP95LatencyMs  float64                            `json:"nominal_p95_latency_ms"`
	BaseOverheadMs       float64                            `json:"base_overhead_ms"`
	InputMsPer1kTokens   float64                            `json:"input_ms_per_1k_tokens"`
	OutputMsPer1kTokens  float64                            `json:"output_ms_per_1k_tokens"`
	ContextWindow        int                                `json:"context_window"`
	SecurityClasses      []string                           `json:"security_classes"`
	TaskQualityMatrix    map[TaskComplexity]float64         `json:"task_quality_matrix"`
	HealthStatus         ModelHealthStatus                  `json:"health_status"`
	ObservedMetrics      ObservedModelMetrics               `json:"observed_metrics"`
}

// ObservedModelMetrics captures live runtime observations from telemetry.
type ObservedModelMetrics struct {
	ObservedP50LatencyMs float64   `json:"observed_p50_latency_ms"`
	ObservedErrorRate    float64   `json:"observed_error_rate"`
	ConsecutiveFailures  int       `json:"consecutive_failures"`
	LastFailureAt        time.Time `json:"last_failure_at,omitempty"`
	LastSuccessAt        time.Time `json:"last_success_at,omitempty"`
	TotalInvocations     int64     `json:"total_invocations"`
}

// ModelRejection documents why a specific candidate model was not selected.
type ModelRejection struct {
	ModelID string `json:"model_id"`
	Reason  string `json:"reason"`
	Details string `json:"details"`
}

// RoutingRequest encapsulates requirements for routing an agent task.
type RoutingRequest struct {
	TaskID                string         `json:"task_id"`
	RunID                 string         `json:"run_id"`
	AgentID               string         `json:"agent_id"`
	TenantID              string         `json:"tenant_id"`
	Prompt                string         `json:"prompt"`
	TaskComplexity        TaskComplexity `json:"task_complexity"`
	QualityRequirement    float64        `json:"quality_requirement"` // Min acceptable [0.0, 1.0]
	LatencySLAMs          float64        `json:"latency_sla_ms,omitempty"`
	CostBudgetUSD         float64        `json:"cost_budget_usd,omitempty"`
	EstimatedInputTokens  int            `json:"estimated_input_tokens"`
	EstimatedOutputTokens int            `json:"estimated_output_tokens"`
	SecurityProfile       string         `json:"security_profile"`
	RoutingPolicy         RoutingPolicy  `json:"routing_policy"`
	PinnedModelID         string         `json:"pinned_model_id,omitempty"`
	RegistryVersion       string         `json:"registry_version,omitempty"`
	PolicyVersion         string         `json:"policy_version,omitempty"`
}

// ScoreBreakdown contains normalized multi-objective component scores.
type ScoreBreakdown struct {
	Quality     float64 `json:"quality"`
	Cost        float64 `json:"cost"`
	Latency     float64 `json:"latency"`
	Reliability float64 `json:"reliability"`
}

// RoutingDecision represents the deterministic result of model selection.
type RoutingDecision struct {
	TaskID              string           `json:"task_id"`
	RunID               string           `json:"run_id"`
	SelectedModelID     string           `json:"selected_model_id"`
	SelectedTier        ModelTier        `json:"selected_tier"`
	Policy              RoutingPolicy    `json:"policy"`
	AlgorithmVersion    string           `json:"algorithm_version"`
	RegistryVersion     string           `json:"registry_version"`
	PolicyVersion       string           `json:"policy_version"`
	EstimatedCostUSD    float64          `json:"estimated_cost_usd"`
	EstimatedLatencyMs  float64          `json:"estimated_latency_ms"`
	QualityScore        float64          `json:"quality_score"`
	FinalScore          float64          `json:"final_score"`
	ScoreBreakdown      ScoreBreakdown   `json:"score_breakdown"`
	FallbackCandidates  []string         `json:"fallback_candidates"`
	Rejections          []ModelRejection `json:"rejections"`
	IsParetoOptimal     bool             `json:"is_pareto_optimal"`
	DecidedAt           time.Time        `json:"decided_at"`
}
