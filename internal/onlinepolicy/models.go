package onlinepolicy

import (
	"time"

	"github.com/sentinelmesh/sentinelmesh/internal/router"
)

// Version constants for Stage 19 Policy Engine.
const (
	PolicyVersion      = "policy-v2.0"
	RewardVersion      = "reward-v1.0"
	ExplorationVersion = "ucb-v1.0"
	GuardrailVersion   = "guardrails-v1.0"
)

// ExecutionMode defines deployment routing behavior.
type ExecutionMode string

const (
	ModeActive ExecutionMode = "ACTIVE" // 100% Policy controlled
	ModeCanary ExecutionMode = "CANARY" // Configurable fraction policy, remainder baseline
	ModeShadow ExecutionMode = "SHADOW" // 100% Baseline execution, policy observes & evaluates
)

// DecisionMode specifies whether a decision was exploratory or exploitative.
type DecisionMode string

const (
	DecisionExploit DecisionMode = "EXPLOIT"
	DecisionExplore DecisionMode = "EXPLORE"
	DecisionCanary  DecisionMode = "CANARY"
	DecisionShadow  DecisionMode = "SHADOW"
)

// RewardWeights configures the multi-objective reward function.
type RewardWeights struct {
	WeightQuality  float64 `json:"weight_quality"`  // w_q
	WeightSuccess  float64 `json:"weight_success"`  // w_s
	WeightCost     float64 `json:"weight_cost"`     // w_c
	WeightLatency  float64 `json:"weight_latency"`  // w_l
	WeightFallback float64 `json:"weight_fallback"` // w_f
}

// DefaultRewardWeights returns standard balanced weights.
func DefaultRewardWeights() RewardWeights {
	return RewardWeights{
		WeightQuality:  0.45,
		WeightSuccess:  0.25,
		WeightCost:     0.15,
		WeightLatency:  0.10,
		WeightFallback: 0.05,
	}
}

// PolicyState encapsulates mutable and versioned policy parameters.
type PolicyState struct {
	Version                  string             `json:"version"`
	ParentVersion            string             `json:"parent_version"`
	Mode                     ExecutionMode      `json:"mode"`
	CanaryFraction           float64            `json:"canary_fraction"`
	ExplorationLambda        float64            `json:"exploration_lambda"` // UCB λ coefficient
	ExplorationBudget        float64            `json:"exploration_budget"` // 0.05 (5%)
	GlobalExplorationLimit   int                `json:"global_exploration_limit"` // e.g. 10 per 200
	PerModelExplorationLimit int                `json:"per_model_exploration_limit"` // e.g. 4 per 200
	WindowSize               int                `json:"window_size"` // 200
	TotalDecisions           uint64             `json:"total_decisions"`
	ExplorationCount         uint64             `json:"exploration_count"`
	ExploitationCount        uint64             `json:"exploitation_count"`
	RewardWeights            RewardWeights      `json:"reward_weights"`
	IsRolledBack             bool               `json:"is_rolled_back"`
	LastRollback             time.Time          `json:"last_rollback"`
	CreatedAt                time.Time          `json:"created_at"`
}

// DefaultPolicyState returns standard Stage 19 configuration.
func DefaultPolicyState() PolicyState {
	return PolicyState{
		Version:                  PolicyVersion,
		ParentVersion:            "policy-v1.0",
		Mode:                     ModeActive,
		CanaryFraction:           0.10,
		ExplorationLambda:        0.50,
		ExplorationBudget:        0.05,
		GlobalExplorationLimit:   10,
		PerModelExplorationLimit: 4,
		WindowSize:               200,
		RewardWeights:            DefaultRewardWeights(),
		CreatedAt:                time.Now().UTC(),
	}
}

// PolicyDecision encapsulates the decision produced by the contextual bandit.
type PolicyDecision struct {
	TaskID             string                  `json:"task_id"`
	RunID              string                  `json:"run_id"`
	SelectedModelID    string                  `json:"selected_model_id"`
	SelectedTier       router.ModelTier        `json:"selected_tier"`
	DecisionMode       DecisionMode            `json:"decision_mode"`
	PolicyVersion      string                  `json:"policy_version"`
	RewardVersion      string                  `json:"reward_version"`
	ExplorationVersion string                  `json:"exploration_version"`
	ExpectedUtility    float64                 `json:"expected_utility"`
	UCBScore           float64                 `json:"ucb_score"`
	Uncertainty        float64                 `json:"uncertainty"`
	ExplorationRate    float64                 `json:"exploration_rate"`
	FallbackCandidates []string                `json:"fallback_candidates"`
	Rejections         []router.ModelRejection `json:"rejections"`
	ScoreBreakdown     router.ScoreBreakdown   `json:"score_breakdown"`
	DecidedAt          time.Time               `json:"decided_at"`
}

// ShadowEvaluation contains comparison metrics when running in Shadow mode.
type ShadowEvaluation struct {
	TaskID               string    `json:"task_id"`
	LiveModelID          string    `json:"live_model_id"`
	ShadowModelID        string    `json:"shadow_model_id"`
	LiveExpectedUtility  float64   `json:"live_expected_utility"`
	ShadowExpectedUtility float64  `json:"shadow_expected_utility"`
	Agreement            bool      `json:"agreement"`
	EvaluatedAt          time.Time `json:"evaluated_at"`
}

// GuardrailConfig defines safety rollback thresholds with hysteresis.
type GuardrailConfig struct {
	MinQualityFloor        float64 `json:"min_quality_floor"`        // Breach if < 0.85
	QualityRecoveryFloor   float64 `json:"quality_recovery_floor"`   // Recover if >= 0.88
	MaxCostIncreasePct     float64 `json:"max_cost_increase_pct"`     // Breach if > 20.0%
	MaxLatencyIncreasePct  float64 `json:"max_latency_increase_pct"`  // Breach if > 25.0%
	MaxFallbackRatePct     float64 `json:"max_fallback_rate_pct"`     // Breach if > 5.0%
	RollingWindowSize      int     `json:"rolling_window_size"`      // 50 decisions
	RecoveryConsecutiveReq int     `json:"recovery_consecutive_req"` // 30 decisions
}

// DefaultGuardrailConfig returns standard hysteresis guardrail parameters.
func DefaultGuardrailConfig() GuardrailConfig {
	return GuardrailConfig{
		MinQualityFloor:        0.85,
		QualityRecoveryFloor:   0.88,
		MaxCostIncreasePct:     20.0,
		MaxLatencyIncreasePct:  25.0,
		MaxFallbackRatePct:     5.0,
		RollingWindowSize:      50,
		RecoveryConsecutiveReq: 30,
	}
}

// RollbackEvent records an automatic policy reversion.
type RollbackEvent struct {
	CurrentVersion string    `json:"current_version"`
	TargetVersion  string    `json:"target_version"`
	TriggerMetric  string    `json:"trigger_metric"`
	ObservedValue  float64   `json:"observed_value"`
	ThresholdValue float64   `json:"threshold_value"`
	Reason         string    `json:"reason"`
	RolledBackAt   time.Time `json:"rolled_back_at"`
}
