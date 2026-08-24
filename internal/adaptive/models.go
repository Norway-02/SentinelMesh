package adaptive

import (
	"time"

	"github.com/sentinelmesh/sentinelmesh/internal/router"
)

// Version constants for adaptive models and state schemas.
const (
	LearningModelVersion = "adaptive-v1.0"
	FeatureSchemaVersion = "features-v1.0"
	PriorVersion         = "prior-v1.0"
	DriftDetectorVersion = "drift-v1.0"
)

// TokenBucket discretizes token counts to prevent combinatorial state space explosion.
type TokenBucket string

const (
	BucketSmall   TokenBucket = "<1k"
	BucketMedium  TokenBucket = "1k-8k"
	BucketLarge   TokenBucket = "8k-32k"
	BucketExtreme TokenBucket = ">32k"
)

// TaskClass classifies task semantic intent.
type TaskClass string

const (
	ClassExtraction    TaskClass = "extraction"
	ClassCodeGen       TaskClass = "codegen"
	ClassSummarization TaskClass = "summarization"
	ClassReasoning     TaskClass = "reasoning"
	ClassGeneral       TaskClass = "general"
)

// FeatureKey uniquely identifies a performance slice in the online learning store.
type FeatureKey struct {
	ModelID           string                `json:"model_id"`
	TaskClass         TaskClass             `json:"task_class"`
	Complexity        router.TaskComplexity `json:"complexity"`
	InputTokenBucket  TokenBucket           `json:"input_token_bucket"`
	OutputTokenBucket TokenBucket           `json:"output_token_bucket"`
}

// IntermediateKey creates a coarser key for hierarchical fallback when sample count is low.
func (k FeatureKey) IntermediateKey() FeatureKey {
	return FeatureKey{
		ModelID:    k.ModelID,
		TaskClass:  k.TaskClass,
		Complexity: k.Complexity,
	}
}

// TaskFeatures encapsulates extracted features from a task request.
type TaskFeatures struct {
	TaskID             string                `json:"task_id"`
	TaskClass          TaskClass             `json:"task_class"`
	Complexity         router.TaskComplexity `json:"complexity"`
	InputTokens        int                   `json:"input_tokens"`
	OutputTokens       int                   `json:"output_tokens"`
	InBucket           TokenBucket           `json:"in_bucket"`
	OutBucket          TokenBucket           `json:"out_bucket"`
	QualityRequirement float64               `json:"quality_requirement"`
	SecurityProfile    string                `json:"security_profile"`
}

// BetaPrior configures Bayesian conjugate prior hyperparameters.
type BetaPrior struct {
	Alpha   float64 `json:"alpha"`
	Beta    float64 `json:"beta"`
	Version string  `json:"version"`
}

// DefaultBetaPrior returns standard prior (mean=0.833, moderate strength).
func DefaultBetaPrior() BetaPrior {
	return BetaPrior{
		Alpha:   5.0,
		Beta:    1.0,
		Version: PriorVersion,
	}
}

// BetaQuantiles represents exact posterior quantiles.
type BetaQuantiles struct {
	Q025 float64 `json:"q025"` // 2.5th percentile
	Q50  float64 `json:"q50"`  // 50th percentile (median)
	Q975 float64 `json:"q975"` // 97.5th percentile
}

// QualityEstimate represents an empirical-Bayes quality prediction.
type QualityEstimate struct {
	Mean     float64 `json:"mean"`
	Variance float64 `json:"variance"`
	Samples  int     `json:"samples"`
	LowerCI  float64 `json:"lower_ci"`
	UpperCI  float64 `json:"upper_ci"`
}

// LatencyRegressionState tracks online least-squares parameters for latency estimation.
type LatencyRegressionState struct {
	Theta0 float64 `json:"theta_0"` // Base overhead ms
	Theta1 float64 `json:"theta_1"` // Ms per input token
	Theta2 float64 `json:"theta_2"` // Ms per output token
}

// LatencyEstimate holds predicted execution latency and observed tail risk.
type LatencyEstimate struct {
	PredictedMs   float64 `json:"predicted_ms"`
	ObservedP50Ms float64 `json:"observed_p50_ms"`
	ObservedP95Ms float64 `json:"observed_p95_ms"`
	Samples       int     `json:"samples"`
}

// CostEstimate captures deterministic base cost and empirical correction ratio.
type CostEstimate struct {
	PredictedUSD     float64 `json:"predicted_usd"`
	NominalUSD       float64 `json:"nominal_usd"`
	CorrectionFactor float64 `json:"correction_factor"`
	Samples          int     `json:"samples"`
}

// PerformanceProfile aggregates online statistics for a specific FeatureKey.
type PerformanceProfile struct {
	Key               FeatureKey             `json:"key"`
	TotalAttempts     int                    `json:"total_attempts"`
	SuccessCount      int                    `json:"success_count"`
	FailureCount      int                    `json:"failure_count"`
	QualitySum        float64                `json:"quality_sum"`
	QualitySqSum      float64                `json:"quality_sq_sum"`
	CostNominalSum    float64                `json:"cost_nominal_sum"`
	CostActualSum     float64                `json:"cost_actual_sum"`
	LatencyRegression LatencyRegressionState `json:"latency_regression"`
	RecentLatencies   []float64              `json:"recent_latencies"`
	RecentQualities   []float64              `json:"recent_qualities"`
	RecentSuccesses   []bool                 `json:"recent_successes"`
	LastUpdated       time.Time              `json:"last_updated"`
}

// AdaptiveRoutingDecision is the output of the predictive adaptive router.
type AdaptiveRoutingDecision struct {
	TaskID               string                  `json:"task_id"`
	RunID                string                  `json:"run_id"`
	SelectedModelID      string                  `json:"selected_model_id"`
	SelectedTier         router.ModelTier        `json:"selected_tier"`
	Policy               router.RoutingPolicy    `json:"policy"`
	EffectiveUtility     float64                 `json:"effective_utility"`
	Confidence           float64                 `json:"confidence"`
	SampleCount          int                     `json:"sample_count"`
	PredictedSuccess     float64                 `json:"predicted_success"`
	SuccessQuantiles     BetaQuantiles           `json:"success_quantiles"`
	QualityEstimate      QualityEstimate         `json:"quality_estimate"`
	LatencyEstimate      LatencyEstimate         `json:"latency_estimate"`
	CostEstimate         CostEstimate            `json:"cost_estimate"`
	NominalScore         float64                 `json:"nominal_score"`
	AdaptiveScore        float64                 `json:"adaptive_score"`
	ScoreBreakdown       router.ScoreBreakdown   `json:"score_breakdown"`
	FallbackCandidates   []string                `json:"fallback_candidates"`
	Rejections           []router.ModelRejection `json:"rejections"`
	LearningModelVersion string                  `json:"learning_model_version"`
	FeatureSchemaVersion string                  `json:"feature_schema_version"`
	PriorVersion         string                  `json:"prior_version"`
	DriftDetectorVersion string                  `json:"drift_detector_version"`
	DecidedAt            time.Time               `json:"decided_at"`
}
