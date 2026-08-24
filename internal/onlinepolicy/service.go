package onlinepolicy

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"time"

	"github.com/google/uuid"
	"github.com/sentinelmesh/sentinelmesh/internal/adaptive"
	"github.com/sentinelmesh/sentinelmesh/internal/events"
	"github.com/sentinelmesh/sentinelmesh/internal/observability"
	"github.com/sentinelmesh/sentinelmesh/internal/outbox"
	"github.com/sentinelmesh/sentinelmesh/internal/repository"
	"github.com/sentinelmesh/sentinelmesh/internal/router"
)

// OnlinePolicyService coordinates safe online policy decisions, execution, reward logging, and guardrails.
type OnlinePolicyService struct {
	registry      router.Registry
	provider      router.ModelProvider
	learningStore adaptive.LearningStore
	prior         adaptive.BetaPrior
	policyManager *PolicyManager
	tracker       *ExplorationTracker
	guardrails    *GuardrailEnforcer
	decisionRepo  router.DecisionRepository
	outboxRepo    outbox.Repository
	txManager     repository.TxManager
	rng           *rand.Rand
	baselineCost  float64
	baselineLatMs float64
}

// NewOnlinePolicyService constructs an OnlinePolicyService.
func NewOnlinePolicyService(
	registry router.Registry,
	provider router.ModelProvider,
	learningStore adaptive.LearningStore,
	prior adaptive.BetaPrior,
	policyManager *PolicyManager,
	guardrails *GuardrailEnforcer,
	decisionRepo router.DecisionRepository,
	outboxRepo outbox.Repository,
	txManager repository.TxManager,
) *OnlinePolicyService {
	if learningStore == nil {
		learningStore = adaptive.NewMemoryLearningStore()
	}
	if prior.Version == "" {
		prior = adaptive.DefaultBetaPrior()
	}
	if policyManager == nil {
		policyManager = NewPolicyManager(DefaultPolicyState())
	}
	if guardrails == nil {
		guardrails = NewGuardrailEnforcer(DefaultGuardrailConfig())
	}
	if decisionRepo == nil {
		decisionRepo = router.NewMemoryDecisionRepository()
	}

	state := policyManager.GetActiveState()
	tracker := NewExplorationTracker(state.WindowSize)

	return &OnlinePolicyService{
		registry:      registry,
		provider:      provider,
		learningStore: learningStore,
		prior:         prior,
		policyManager: policyManager,
		tracker:       tracker,
		guardrails:    guardrails,
		decisionRepo:  decisionRepo,
		outboxRepo:    outboxRepo,
		txManager:     txManager,
		rng:           rand.New(rand.NewSource(time.Now().UnixNano())),
		baselineCost:  0.0015,
		baselineLatMs: 200.0,
	}
}

// SetBaselines sets reference baseline cost and latency for guardrail evaluations.
func (s *OnlinePolicyService) SetBaselines(cost float64, latMs float64) {
	s.baselineCost = cost
	s.baselineLatMs = latMs
}

// GetPolicyManager returns the underlying policy manager.
func (s *OnlinePolicyService) GetPolicyManager() *PolicyManager {
	return s.policyManager
}

// Route makes a policy decision obeying the immutable Stage 17 safe feasible set.
func (s *OnlinePolicyService) Route(ctx context.Context, req router.RoutingRequest) (PolicyDecision, error) {
	ctx, span := observability.StartSpan(ctx, "onlinepolicy.route")
	defer span.End()

	models, err := s.registry.ListModels(ctx)
	if err != nil {
		return PolicyDecision{}, fmt.Errorf("failed to list models: %w", err)
	}

	// 1. Establish Inviolable Stage 17 Feasible Set
	nominalDecision, err := router.RouteTask(req, models)
	if err != nil {
		return PolicyDecision{
			TaskID:             req.TaskID,
			RunID:              req.RunID,
			PolicyVersion:      s.policyManager.GetActiveState().Version,
			RewardVersion:      RewardVersion,
			ExplorationVersion: ExplorationVersion,
			Rejections:         nominalDecision.Rejections,
			DecidedAt:          time.Now().UTC(),
		}, fmt.Errorf("hard constraint failure: %w", err)
	}

	feasibleMap := make(map[string]bool)
	feasibleMap[nominalDecision.SelectedModelID] = true
	for _, fb := range nominalDecision.FallbackCandidates {
		feasibleMap[fb] = true
	}

	var feasibleModels []router.ModelDefinition
	for _, m := range models {
		if feasibleMap[m.ID] {
			feasibleModels = append(feasibleModels, m)
		}
	}

	state := s.policyManager.GetActiveState()

	// 2. Handle Deployment Execution Modes (Active, Canary, Shadow)
	var decision PolicyDecision

	switch state.Mode {
	case ModeShadow:
		// Live execution uses nominal/adaptive baseline, Shadow evaluates bandit
		decision, _ = SelectArm(req, feasibleModels, nominalDecision.Rejections, s.learningStore, s.prior, &state, s.tracker, s.rng)
		decision.DecisionMode = DecisionShadow
		s.publishShadowEvaluation(ctx, req, nominalDecision.SelectedModelID, decision.SelectedModelID, 0.90, decision.ExpectedUtility)
		decision.SelectedModelID = nominalDecision.SelectedModelID // Enforce live baseline execution

	case ModeCanary:
		if s.rng.Float64() < state.CanaryFraction {
			decision, err = SelectArm(req, feasibleModels, nominalDecision.Rejections, s.learningStore, s.prior, &state, s.tracker, s.rng)
			if err == nil {
				decision.DecisionMode = DecisionCanary
			}
		} else {
			decision, err = SelectArm(req, feasibleModels, nominalDecision.Rejections, s.learningStore, s.prior, &state, s.tracker, s.rng)
			if err == nil {
				decision.DecisionMode = DecisionExploit
				decision.SelectedModelID = nominalDecision.SelectedModelID
			}
		}

	case ModeActive:
		fallthrough
	default:
		decision, err = SelectArm(req, feasibleModels, nominalDecision.Rejections, s.learningStore, s.prior, &state, s.tracker, s.rng)
	}

	if err != nil {
		observability.RecordSpanError(span, err)
		return PolicyDecision{}, err
	}

	s.publishPolicyDecisionEvent(ctx, req, decision)
	return decision, nil
}

// Execute evaluates policy, invokes model, computes reward, updates learning store, and verifies guardrails.
func (s *OnlinePolicyService) Execute(ctx context.Context, req router.RoutingRequest) (*router.ModelInvocationResponse, error) {
	ctx, span := observability.StartSpan(ctx, "onlinepolicy.execute")
	defer span.End()

	decision, err := s.Route(ctx, req)
	if err != nil {
		return nil, err
	}

	candidates := append([]string{decision.SelectedModelID}, decision.FallbackCandidates...)
	invReq := router.ModelInvocationRequest{
		TaskID:         req.TaskID,
		RunID:          req.RunID,
		AgentID:        req.AgentID,
		TenantID:       req.TenantID,
		Prompt:         req.Prompt,
		TaskComplexity: req.TaskComplexity,
		MaxTokens:      req.EstimatedOutputTokens,
	}

	var lastErr error
	for attempt, modelID := range candidates {
		resp, err := s.provider.Invoke(ctx, modelID, invReq)
		if err != nil {
			lastErr = err
			// Record failure outcome
			outcome := router.RoutingOutcomeRecord{
				TaskID:             req.TaskID,
				RunID:              req.RunID,
				ModelID:            modelID,
				ActualLatency:      5 * time.Second,
				ActualCostUSD:      0.0,
				ActualQualityScore: 0.0,
				Success:            false,
				FallbackUsed:       (attempt > 0),
				AttemptNumber:      attempt + 1,
				CompletedAt:        time.Now().UTC(),
			}
			_ = s.learningStore.RecordOutcome(ctx, outcome, req)
			continue
		}

		resp.FallbackUsed = (attempt > 0)
		resp.AttemptNumber = attempt + 1

		// 1. Ingest into Learning Store
		outcome := router.RoutingOutcomeRecord{
			TaskID:             req.TaskID,
			RunID:              req.RunID,
			ModelID:            modelID,
			ActualLatency:      resp.ActualLatency,
			ActualCostUSD:      resp.ActualCostUSD,
			ActualQualityScore: resp.QualityScore,
			Success:            true,
			FallbackUsed:       resp.FallbackUsed,
			AttemptNumber:      resp.AttemptNumber,
			CompletedAt:        time.Now().UTC(),
		}
		_ = s.learningStore.RecordOutcome(ctx, outcome, req)

		// 2. Compute Scalar Reward
		state := s.policyManager.GetActiveState()
		_ = CalculateReward(outcome, req, state.RewardWeights)

		// 3. Evaluate Guardrails & Automatic Policy Rollback
		breached, rollbackEvent := s.guardrails.RecordOutcome(outcome, s.baselineCost, s.baselineLatMs)
		if breached {
			_, _ = s.policyManager.TriggerRollback(rollbackEvent)
			s.publishRollbackEvent(ctx, rollbackEvent)
		}

		// 4. Persist Decision Telemetry
		_ = s.decisionRepo.RecordOutcome(ctx, outcome)

		return resp, nil
	}

	return nil, fmt.Errorf("all feasible policy candidates exhausted, last error: %w", lastErr)
}

func (s *OnlinePolicyService) publishPolicyDecisionEvent(ctx context.Context, req router.RoutingRequest, d PolicyDecision) {
	if s.outboxRepo == nil {
		return
	}

	payload, _ := json.Marshal(events.OnlinePolicyDecidedPayload{
		TaskID:             d.TaskID,
		RunID:              d.RunID,
		AgentID:            req.AgentID,
		TenantID:           req.TenantID,
		SelectedModelID:    d.SelectedModelID,
		DecisionMode:       string(d.DecisionMode),
		PolicyVersion:      d.PolicyVersion,
		RewardVersion:      d.RewardVersion,
		ExplorationVersion: d.ExplorationVersion,
		ExpectedUtility:    d.ExpectedUtility,
		UCBScore:           d.UCBScore,
		Uncertainty:        d.Uncertainty,
		ExplorationRate:    d.ExplorationRate,
		FallbackCandidates: d.FallbackCandidates,
		ScoreBreakdown: map[string]float64{
			"quality":     d.ScoreBreakdown.Quality,
			"cost":        d.ScoreBreakdown.Cost,
			"latency":     d.ScoreBreakdown.Latency,
			"reliability": d.ScoreBreakdown.Reliability,
		},
		DecidedAt: d.DecidedAt,
	})

	evt := events.Event{
		EventID:       uuid.New().String(),
		EventType:     events.SubjectOnlinePolicyDecided,
		SchemaVersion: 1,
		AggregateType: "OnlinePolicy",
		AggregateID:   d.TaskID,
		TenantID:      req.TenantID,
		OccurredAt:    time.Now().UTC(),
		Payload:       payload,
	}

	_ = s.outboxRepo.Insert(ctx, evt)
}

func (s *OnlinePolicyService) publishRollbackEvent(ctx context.Context, r RollbackEvent) {
	if s.outboxRepo == nil {
		return
	}

	payload, _ := json.Marshal(events.PolicyRollbackPayload{
		CurrentVersion: r.CurrentVersion,
		TargetVersion:  r.TargetVersion,
		TriggerMetric:  r.TriggerMetric,
		ObservedValue:  r.ObservedValue,
		ThresholdValue: r.ThresholdValue,
		Reason:         r.Reason,
		RolledBackAt:   r.RolledBackAt,
	})

	evt := events.Event{
		EventID:       uuid.New().String(),
		EventType:     events.SubjectPolicyRollbackTriggered,
		SchemaVersion: 1,
		AggregateType: "OnlinePolicy",
		AggregateID:   r.CurrentVersion,
		TenantID:      "system",
		OccurredAt:    time.Now().UTC(),
		Payload:       payload,
	}

	_ = s.outboxRepo.Insert(ctx, evt)
}

func (s *OnlinePolicyService) publishShadowEvaluation(ctx context.Context, req router.RoutingRequest, liveID, shadowID string, liveUtil, shadowUtil float64) {
	if s.outboxRepo == nil {
		return
	}

	payload, _ := json.Marshal(events.ShadowPolicyEvaluatedPayload{
		TaskID:               req.TaskID,
		LiveModelID:          liveID,
		ShadowModelID:        shadowID,
		LiveExpectedReward:   liveUtil,
		ShadowExpectedReward: shadowUtil,
		Agreement:            (liveID == shadowID),
		EvaluatedAt:          time.Now().UTC(),
	})

	evt := events.Event{
		EventID:       uuid.New().String(),
		EventType:     events.SubjectShadowPolicyEvaluated,
		SchemaVersion: 1,
		AggregateType: "OnlinePolicy",
		AggregateID:   req.TaskID,
		TenantID:      req.TenantID,
		OccurredAt:    time.Now().UTC(),
		Payload:       payload,
	}

	_ = s.outboxRepo.Insert(ctx, evt)
}
