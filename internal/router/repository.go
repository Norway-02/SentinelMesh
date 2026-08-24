package router

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// RoutingDecisionRecord represents a persisted routing decision.
type RoutingDecisionRecord struct {
	TaskID             string           `json:"task_id"`
	RunID              string           `json:"run_id"`
	SelectedModelID    string           `json:"selected_model_id"`
	SelectedTier       ModelTier        `json:"selected_tier"`
	Policy             RoutingPolicy    `json:"policy"`
	AlgorithmVersion   string           `json:"algorithm_version"`
	RegistryVersion    string           `json:"registry_version"`
	PolicyVersion      string           `json:"policy_version"`
	EstimatedCostUSD   float64          `json:"estimated_cost_usd"`
	EstimatedLatencyMs float64          `json:"estimated_latency_ms"`
	PredictedQuality   float64          `json:"predicted_quality"`
	FinalScore         float64          `json:"final_score"`
	FallbackCandidates []string         `json:"fallback_candidates"`
	ScoreBreakdown     ScoreBreakdown   `json:"score_breakdown"`
	Rejections         []ModelRejection `json:"rejections"`
	IsParetoOptimal    bool             `json:"is_pareto_optimal"`
	CreatedAt          time.Time        `json:"created_at"`
}

// RoutingOutcomeRecord captures actual execution metrics for feedback and Stage 18 ML training.
type RoutingOutcomeRecord struct {
	TaskID             string        `json:"task_id"`
	RunID              string        `json:"run_id"`
	ModelID            string        `json:"model_id"`
	ActualLatency      time.Duration `json:"actual_latency"`
	ActualCostUSD      float64       `json:"actual_cost_usd"`
	ActualQualityScore float64       `json:"actual_quality_score"`
	Success            bool          `json:"success"`
	FallbackUsed       bool          `json:"fallback_used"`
	AttemptNumber      int           `json:"attempt_number"`
	CompletedAt        time.Time     `json:"completed_at"`
}

// DecisionRepository defines storage for routing decisions and outcome telemetry.
type DecisionRepository interface {
	SaveDecision(ctx context.Context, record RoutingDecisionRecord) error
	GetDecision(ctx context.Context, taskID string) (RoutingDecisionRecord, error)
	RecordOutcome(ctx context.Context, record RoutingOutcomeRecord) error
	ListDecisions(ctx context.Context, limit int) ([]RoutingDecisionRecord, error)
	ListOutcomes(ctx context.Context, limit int) ([]RoutingOutcomeRecord, error)
}

// MemoryDecisionRepository provides in-memory thread-safe storage.
type MemoryDecisionRepository struct {
	mu        sync.RWMutex
	decisions map[string]RoutingDecisionRecord
	outcomes  []RoutingOutcomeRecord
}

// NewMemoryDecisionRepository initializes MemoryDecisionRepository.
func NewMemoryDecisionRepository() *MemoryDecisionRepository {
	return &MemoryDecisionRepository{
		decisions: make(map[string]RoutingDecisionRecord),
		outcomes:  make([]RoutingOutcomeRecord, 0),
	}
}

// SaveDecision stores a routing decision record.
func (r *MemoryDecisionRepository) SaveDecision(ctx context.Context, record RoutingDecisionRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.decisions[record.TaskID] = record
	return nil
}

// GetDecision retrieves a decision by TaskID.
func (r *MemoryDecisionRepository) GetDecision(ctx context.Context, taskID string) (RoutingDecisionRecord, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	d, ok := r.decisions[taskID]
	if !ok {
		return RoutingDecisionRecord{}, fmt.Errorf("decision for task %s not found", taskID)
	}
	return d, nil
}

// RecordOutcome appends actual runtime execution results.
func (r *MemoryDecisionRepository) RecordOutcome(ctx context.Context, record RoutingOutcomeRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.outcomes = append(r.outcomes, record)
	return nil
}

// ListDecisions returns recently recorded decisions up to limit.
func (r *MemoryDecisionRepository) ListDecisions(ctx context.Context, limit int) ([]RoutingDecisionRecord, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var list []RoutingDecisionRecord
	for _, d := range r.decisions {
		list = append(list, d)
		if limit > 0 && len(list) >= limit {
			break
		}
	}
	return list, nil
}

// ListOutcomes returns recent outcomes up to limit.
func (r *MemoryDecisionRepository) ListOutcomes(ctx context.Context, limit int) ([]RoutingOutcomeRecord, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if limit <= 0 || limit >= len(r.outcomes) {
		copied := make([]RoutingOutcomeRecord, len(r.outcomes))
		copy(copied, r.outcomes)
		return copied, nil
	}

	start := len(r.outcomes) - limit
	copied := make([]RoutingOutcomeRecord, limit)
	copy(copied, r.outcomes[start:])
	return copied, nil
}
