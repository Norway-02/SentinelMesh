package adaptive

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
	"github.com/sentinelmesh/sentinelmesh/internal/router"
)

// AdaptiveService orchestrates predictive model selection, learning store updates, and drift monitoring.
type AdaptiveService struct {
	registry     router.Registry
	provider     router.ModelProvider
	store        LearningStore
	detector     *DualWindowDriftDetector
	prior        BetaPrior
	decisionRepo router.DecisionRepository
	outboxRepo   outbox.Repository
	txManager    repository.TxManager
}

// NewAdaptiveService constructs a new AdaptiveService.
func NewAdaptiveService(
	registry router.Registry,
	provider router.ModelProvider,
	store LearningStore,
	detector *DualWindowDriftDetector,
	prior BetaPrior,
	decisionRepo router.DecisionRepository,
	outboxRepo outbox.Repository,
	txManager repository.TxManager,
) *AdaptiveService {
	if store == nil {
		store = NewMemoryLearningStore()
	}
	if detector == nil {
		detector = NewDualWindowDriftDetector()
	}
	if prior.Version == "" {
		prior = DefaultBetaPrior()
	}
	if decisionRepo == nil {
		decisionRepo = router.NewMemoryDecisionRepository()
	}

	return &AdaptiveService{
		registry:     registry,
		provider:     provider,
		store:        store,
		detector:     detector,
		prior:        prior,
		decisionRepo: decisionRepo,
		outboxRepo:   outboxRepo,
		txManager:    txManager,
	}
}

// Route computes the adaptive model routing decision without dispatching network invocation.
func (s *AdaptiveService) Route(ctx context.Context, req router.RoutingRequest) (AdaptiveRoutingDecision, error) {
	ctx, span := observability.StartSpan(ctx, "adaptive.route")
	defer span.End()

	models, err := s.registry.ListModels(ctx)
	if err != nil {
		return AdaptiveRoutingDecision{}, fmt.Errorf("failed to list models: %w", err)
	}

	decision, err := RouteAdaptive(req, models, s.store, s.detector, s.prior)
	if err != nil {
		observability.RecordSpanError(span, err)
		return AdaptiveRoutingDecision{}, err
	}

	s.publishAdaptiveRoutingEvent(ctx, req, decision)
	return decision, nil
}

// Execute evaluates predictions, invokes the optimal model, updates the learning store, and checks for drift.
func (s *AdaptiveService) Execute(ctx context.Context, req router.RoutingRequest) (*router.ModelInvocationResponse, error) {
	ctx, span := observability.StartSpan(ctx, "adaptive.execute")
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
			// Record failure in drift detector & learning store
			s.detector.RecordObservation(modelID, 0.0, 5000.0, false)
			_ = s.store.RecordOutcome(ctx, router.RoutingOutcomeRecord{
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
			}, req)
			continue
		}

		// Success path
		resp.FallbackUsed = (attempt > 0)
		resp.AttemptNumber = attempt + 1
		latMs := float64(resp.ActualLatency) / float64(time.Millisecond)

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
		_ = s.store.RecordOutcome(ctx, outcome, req)

		// 2. Feed Dual-Window Drift Detector
		driftDetected, metric, reason := s.detector.RecordObservation(modelID, resp.QualityScore, latMs, true)
		if driftDetected {
			s.publishDriftEvent(ctx, modelID, metric, reason)
		}

		// 3. Persist Telemetry
		_ = s.decisionRepo.RecordOutcome(ctx, outcome)

		return resp, nil
	}

	return nil, fmt.Errorf("all adaptive candidates exhausted, last error: %w", lastErr)
}

func (s *AdaptiveService) publishAdaptiveRoutingEvent(ctx context.Context, req router.RoutingRequest, d AdaptiveRoutingDecision) {
	if s.outboxRepo == nil {
		return
	}

	payload, _ := json.Marshal(events.AdaptiveRoutingDecidedPayload{
		TaskID:               d.TaskID,
		RunID:                d.RunID,
		AgentID:              req.AgentID,
		TenantID:             req.TenantID,
		SelectedModelID:      d.SelectedModelID,
		SelectedTier:         string(d.SelectedTier),
		Policy:               string(d.Policy),
		LearningModelVersion: d.LearningModelVersion,
		FeatureSchemaVersion: d.FeatureSchemaVersion,
		PriorVersion:         d.PriorVersion,
		DriftDetectorVersion: d.DriftDetectorVersion,
		Confidence:           d.Confidence,
		SampleCount:          d.SampleCount,
		PredictedSuccess:     d.PredictedSuccess,
		SuccessLowerCI:       d.SuccessQuantiles.Q025,
		SuccessUpperCI:       d.SuccessQuantiles.Q975,
		PredictedQuality:     d.QualityEstimate.Mean,
		QualityVariance:      d.QualityEstimate.Variance,
		PredictedLatencyMs:   d.LatencyEstimate.PredictedMs,
		PredictedP95LatMs:    d.LatencyEstimate.ObservedP95Ms,
		PredictedCostUSD:     d.CostEstimate.PredictedUSD,
		EffectiveUtility:     d.EffectiveUtility,
		FallbackCandidates:   d.FallbackCandidates,
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
		EventType:     events.SubjectAdaptiveRoutingDecided,
		SchemaVersion: 1,
		AggregateType: "AdaptiveRouter",
		AggregateID:   d.TaskID,
		TenantID:      req.TenantID,
		OccurredAt:    time.Now().UTC(),
		Payload:       payload,
	}

	_ = s.outboxRepo.Insert(ctx, evt)
}

func (s *AdaptiveService) publishDriftEvent(ctx context.Context, modelID, metric, reason string) {
	if s.outboxRepo == nil {
		return
	}

	payload, _ := json.Marshal(events.ModelPerformanceDriftPayload{
		ModelID:         modelID,
		Metric:          metric,
		BaselineValue:   0.90,
		RecentValue:     0.60,
		DeltaPercentage: 30.0,
		Threshold:       15.0,
		ActionTaken:     "penalized_utility",
		DetectedAt:      time.Now().UTC(),
	})

	evt := events.Event{
		EventID:       uuid.New().String(),
		EventType:     events.SubjectModelPerformanceDriftDetected,
		SchemaVersion: 1,
		AggregateType: "AdaptiveRouter",
		AggregateID:   modelID,
		TenantID:      "system",
		OccurredAt:    time.Now().UTC(),
		Payload:       payload,
	}

	_ = s.outboxRepo.Insert(ctx, evt)
}
