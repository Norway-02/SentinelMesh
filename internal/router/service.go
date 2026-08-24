package router

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/sentinelmesh/sentinelmesh/internal/events"
	"github.com/sentinelmesh/sentinelmesh/internal/observability"
	"github.com/sentinelmesh/sentinelmesh/internal/outbox"
	"github.com/sentinelmesh/sentinelmesh/internal/repository"
)

// Service orchestrates model selection, resilient execution, and outbox event publishing.
type Service struct {
	registry     Registry
	provider     ModelProvider
	decisionRepo DecisionRepository
	breakers     *BreakerRegistry
	outboxRepo   outbox.Repository
	txManager    repository.TxManager
}

// NewService constructs a new Model Router Service.
func NewService(
	registry Registry,
	provider ModelProvider,
	decisionRepo DecisionRepository,
	outboxRepo outbox.Repository,
	txManager repository.TxManager,
) *Service {
	if decisionRepo == nil {
		decisionRepo = NewMemoryDecisionRepository()
	}
	return &Service{
		registry:     registry,
		provider:     provider,
		decisionRepo: decisionRepo,
		breakers:     NewBreakerRegistry(),
		outboxRepo:   outboxRepo,
		txManager:    txManager,
	}
}

// Route evaluates the optimal model for a task without invoking the backend.
func (s *Service) Route(ctx context.Context, req RoutingRequest) (RoutingDecision, error) {
	ctx, span := observability.StartSpan(ctx, "router.route")
	defer span.End()

	models, err := s.registry.ListModels(ctx)
	if err != nil {
		observability.RecordSpanError(span, err)
		return RoutingDecision{}, fmt.Errorf("failed to list models: %w", err)
	}

	// Update live health status from circuit breakers
	for i := range models {
		cb := s.breakers.GetOrCreate(models[i].ID)
		models[i].HealthStatus = cb.HealthStatus()
		models[i].ObservedMetrics = cb.Metrics()
	}

	decision, err := RouteTask(req, models)
	if err != nil {
		observability.RecordSpanError(span, err)
		return RoutingDecision{}, err
	}

	// Persist decision record
	record := RoutingDecisionRecord{
		TaskID:             decision.TaskID,
		RunID:              decision.RunID,
		SelectedModelID:    decision.SelectedModelID,
		SelectedTier:       decision.SelectedTier,
		Policy:             decision.Policy,
		AlgorithmVersion:   decision.AlgorithmVersion,
		RegistryVersion:    decision.RegistryVersion,
		PolicyVersion:      decision.PolicyVersion,
		EstimatedCostUSD:   decision.EstimatedCostUSD,
		EstimatedLatencyMs: decision.EstimatedLatencyMs,
		PredictedQuality:   decision.QualityScore,
		FinalScore:         decision.FinalScore,
		FallbackCandidates: decision.FallbackCandidates,
		ScoreBreakdown:     decision.ScoreBreakdown,
		Rejections:         decision.Rejections,
		IsParetoOptimal:    decision.IsParetoOptimal,
		CreatedAt:          decision.DecidedAt,
	}
	_ = s.decisionRepo.SaveDecision(ctx, record)

	// Publish ModelRoutingDecided Event via Outbox
	s.publishRoutingDecidedEvent(ctx, req, decision)

	return decision, nil
}

// Execute routes a task and dispatches inference with automatic circuit breaker fallback.
func (s *Service) Execute(ctx context.Context, req RoutingRequest) (*ModelInvocationResponse, error) {
	ctx, span := observability.StartSpan(ctx, "router.execute")
	defer span.End()

	decision, err := s.Route(ctx, req)
	if err != nil {
		return nil, err
	}

	candidates := append([]string{decision.SelectedModelID}, decision.FallbackCandidates...)
	invReq := ModelInvocationRequest{
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
		cb := s.breakers.GetOrCreate(modelID)
		if !cb.AllowExecution() {
			lastErr = fmt.Errorf("circuit breaker open for model %s", modelID)
			continue
		}

		resp, err := s.provider.Invoke(ctx, modelID, invReq)
		if err != nil {
			lastErr = err
			cb.RecordFailure(err)

			if attempt+1 < len(candidates) {
				s.publishFallbackEvent(ctx, req, modelID, candidates[attempt+1], attempt+1, err.Error())
			}
			continue
		}

		// Success path
		cb.RecordSuccess()
		resp.FallbackUsed = (attempt > 0)
		resp.AttemptNumber = attempt + 1

		// Record runtime outcome telemetry
		_ = s.decisionRepo.RecordOutcome(ctx, RoutingOutcomeRecord{
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
		})

		s.publishInvocationCompletedEvent(ctx, req, resp)
		return resp, nil
	}

	s.publishInvocationFailedEvent(ctx, req, decision.SelectedModelID, lastErr)
	return nil, fmt.Errorf("all candidate models exhausted, last error: %w", lastErr)
}

func (s *Service) publishRoutingDecidedEvent(ctx context.Context, req RoutingRequest, d RoutingDecision) {
	if s.outboxRepo == nil {
		return
	}
	payload, _ := json.Marshal(events.ModelRoutingDecidedPayload{
		TaskID:             d.TaskID,
		RunID:              d.RunID,
		AgentID:            req.AgentID,
		TenantID:           req.TenantID,
		SelectedModelID:    d.SelectedModelID,
		SelectedTier:       string(d.SelectedTier),
		Policy:             string(d.Policy),
		EstimatedCostUSD:   d.EstimatedCostUSD,
		EstimatedLatencyMs: d.EstimatedLatencyMs,
		PredictedQuality:   d.QualityScore,
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
		EventType:     events.SubjectModelRoutingDecided,
		SchemaVersion: 1,
		AggregateType: "ModelRouter",
		AggregateID:   d.TaskID,
		TenantID:      req.TenantID,
		OccurredAt:    time.Now().UTC(),
		Payload:       payload,
	}

	if s.txManager != nil {
		_ = s.txManager.WithinTx(ctx, func(txCtx context.Context) error {
			return s.outboxRepo.Insert(txCtx, evt)
		})
	} else {
		_ = s.outboxRepo.Insert(ctx, evt)
	}
}

func (s *Service) publishFallbackEvent(ctx context.Context, req RoutingRequest, failedModel, fallbackModel string, attempt int, reason string) {
	if s.outboxRepo == nil {
		return
	}
	payload, _ := json.Marshal(events.ModelFallbackTriggeredPayload{
		TaskID:          req.TaskID,
		RunID:           req.RunID,
		FailedModelID:   failedModel,
		FallbackModelID: fallbackModel,
		AttemptNumber:   attempt,
		Reason:          reason,
		TriggeredAt:     time.Now().UTC(),
	})

	evt := events.Event{
		EventID:       uuid.New().String(),
		EventType:     events.SubjectModelFallbackTriggered,
		SchemaVersion: 1,
		AggregateType: "ModelRouter",
		AggregateID:   req.TaskID,
		TenantID:      req.TenantID,
		OccurredAt:    time.Now().UTC(),
		Payload:       payload,
	}
	_ = s.outboxRepo.Insert(ctx, evt)
}

func (s *Service) publishInvocationCompletedEvent(ctx context.Context, req RoutingRequest, resp *ModelInvocationResponse) {
	if s.outboxRepo == nil {
		return
	}
	payload, _ := json.Marshal(events.ModelInvocationCompletedPayload{
		TaskID:         resp.TaskID,
		RunID:          resp.RunID,
		ModelID:        resp.ModelID,
		ActualDuration: resp.ActualLatency,
		ActualCostUSD:  resp.ActualCostUSD,
		PromptTokens:   resp.PromptTokens,
		OutputTokens:   resp.CompletionTokens,
		QualityScore:   resp.QualityScore,
		FallbackUsed:   resp.FallbackUsed,
		CompletedAt:    time.Now().UTC(),
	})

	evt := events.Event{
		EventID:       uuid.New().String(),
		EventType:     events.SubjectModelInvocationCompleted,
		SchemaVersion: 1,
		AggregateType: "ModelRouter",
		AggregateID:   resp.TaskID,
		TenantID:      req.TenantID,
		OccurredAt:    time.Now().UTC(),
		Payload:       payload,
	}
	_ = s.outboxRepo.Insert(ctx, evt)
}

func (s *Service) publishInvocationFailedEvent(ctx context.Context, req RoutingRequest, modelID string, err error) {
	if s.outboxRepo == nil {
		return
	}
	errStr := "unknown error"
	if err != nil {
		errStr = err.Error()
	}
	payload, _ := json.Marshal(events.ModelInvocationFailedPayload{
		TaskID:    req.TaskID,
		RunID:     req.RunID,
		ModelID:   modelID,
		ErrorCode: "MODEL_INVOCATION_FAILED",
		Reason:    errStr,
		FailedAt:  time.Now().UTC(),
	})

	evt := events.Event{
		EventID:       uuid.New().String(),
		EventType:     events.SubjectModelInvocationFailed,
		SchemaVersion: 1,
		AggregateType: "ModelRouter",
		AggregateID:   req.TaskID,
		TenantID:      req.TenantID,
		OccurredAt:    time.Now().UTC(),
		Payload:       payload,
	}
	_ = s.outboxRepo.Insert(ctx, evt)
}
