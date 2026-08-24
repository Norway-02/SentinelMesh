package adaptive_test

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/sentinelmesh/sentinelmesh/internal/adaptive"
	"github.com/sentinelmesh/sentinelmesh/internal/outbox"
	"github.com/sentinelmesh/sentinelmesh/internal/repository/memory"
	"github.com/sentinelmesh/sentinelmesh/internal/router"
)

func setupAdaptiveTest() (*adaptive.AdaptiveService, *router.MemoryRegistry, *router.SyntheticModelProvider, *adaptive.MemoryLearningStore, *adaptive.DualWindowDriftDetector) {
	registry := router.NewDefaultModelRegistry()
	provider := router.NewSyntheticModelProvider(registry, false)
	store := adaptive.NewMemoryLearningStore()
	detector := adaptive.NewDualWindowDriftDetector()
	prior := adaptive.DefaultBetaPrior()
	decisionRepo := router.NewMemoryDecisionRepository()
	outboxRepo := outbox.NewMemoryRepository()
	txManager := memory.NewTxManager()

	svc := adaptive.NewAdaptiveService(registry, provider, store, detector, prior, decisionRepo, outboxRepo, txManager)
	return svc, registry, provider, store, detector
}

func TestAdaptiveRouter_NeverViolatesStage17Constraints(t *testing.T) {
	registry := router.NewDefaultModelRegistry()
	ctx := context.Background()
	models, _ := registry.ListModels(ctx)
	store := adaptive.NewMemoryLearningStore()
	detector := adaptive.NewDualWindowDriftDetector()
	prior := adaptive.DefaultBetaPrior()

	rng := rand.New(rand.NewSource(42))
	secProfiles := []string{"standard", "airgapped", "restricted", "public"}
	complexities := []router.TaskComplexity{
		router.ComplexitySimple,
		router.ComplexityModerate,
		router.ComplexityComplex,
		router.ComplexityReasoningHeavy,
	}
	policies := []router.RoutingPolicy{
		router.PolicyCostOptimized,
		router.PolicyLatencyOptimized,
		router.PolicyQualityOptimized,
		router.PolicyBalanced,
	}

	// 1,000 randomized property-based tests
	for i := 0; i < 1000; i++ {
		req := router.RoutingRequest{
			TaskID:                fmt.Sprintf("prop-task-%d", i),
			RunID:                 fmt.Sprintf("prop-run-%d", i),
			Prompt:                "Property-based invariant test prompt",
			TaskComplexity:        complexities[rng.Intn(len(complexities))],
			QualityRequirement:    0.50 + rng.Float64()*0.45,
			EstimatedInputTokens:  100 + rng.Intn(40000),
			EstimatedOutputTokens: 50 + rng.Intn(10000),
			SecurityProfile:       secProfiles[rng.Intn(len(secProfiles))],
			RoutingPolicy:         policies[rng.Intn(len(policies))],
		}

		// Stage 17 Baseline Decision
		nominalDecision, nomErr := router.RouteTask(req, models)

		// Stage 18 Adaptive Decision
		adaptDecision, adaptErr := adaptive.RouteAdaptive(req, models, store, detector, prior)

		// If Stage 17 rejects, Stage 18 MUST also reject (Safety Invariance)
		if nomErr != nil {
			if adaptErr == nil {
				t.Fatalf("SAFETY VIOLATION on iteration %d: Stage 17 rejected request (%v) but Stage 18 allowed model %s",
					i, nomErr, adaptDecision.SelectedModelID)
			}
			continue
		}

		if adaptErr != nil {
			t.Fatalf("Unexpected error in adaptive router: %v", adaptErr)
		}

		// Feasible set from Stage 17
		feasible := make(map[string]bool)
		feasible[nominalDecision.SelectedModelID] = true
		for _, fb := range nominalDecision.FallbackCandidates {
			feasible[fb] = true
		}

		// Invariant: Selected model MUST be inside Stage 17 Feasible Set
		if !feasible[adaptDecision.SelectedModelID] {
			t.Fatalf("SAFETY INVARIANT BROKEN on iteration %d: Selected model %s is NOT in Stage 17 feasible set %v",
				i, adaptDecision.SelectedModelID, feasible)
		}
	}
}

func TestAdaptive_PredictorCalibrations(t *testing.T) {
	t.Run("TestSuccessPredictorCalibration", func(t *testing.T) {
		profile := adaptive.PerformanceProfile{
			TotalAttempts: 100,
			SuccessCount:  80,
			FailureCount:  20,
		}
		prior := adaptive.DefaultBetaPrior()
		mean, quantiles := adaptive.PredictSuccess(profile, prior)

		// With 80 successes / 20 failures + prior (5, 1):
		// a = 85, b = 21 -> mean = 85/106 = 0.8018
		if mean < 0.78 || mean > 0.82 {
			t.Errorf("Expected posterior mean ~0.80, got %f", mean)
		}
		if quantiles.Q025 >= quantiles.Q50 || quantiles.Q50 >= quantiles.Q975 {
			t.Errorf("Invalid quantile ordering: Q025=%f, Q50=%f, Q975=%f", quantiles.Q025, quantiles.Q50, quantiles.Q975)
		}
		if quantiles.Q025 > mean || quantiles.Q975 < mean {
			t.Errorf("Mean %f not bracketed by 95%% credible interval [%f, %f]", mean, quantiles.Q025, quantiles.Q975)
		}
	})

	t.Run("TestQualityShrinkageCalibration", func(t *testing.T) {
		// True observed quality 0.90 over 50 samples, nominal catalog prior 0.70
		profile := adaptive.PerformanceProfile{
			TotalAttempts: 50,
			QualitySum:    45.0, // mean = 0.90
			QualitySqSum:  40.55,
		}
		est := adaptive.PredictQuality(profile, 0.70)

		// Shrinkage: (50/(50+10))*0.90 + (10/(50+10))*0.70 = (5/6)*0.90 + (1/6)*0.70 = 0.75 + 0.1167 = 0.8667
		if est.Mean < 0.85 || est.Mean > 0.88 {
			t.Errorf("Expected shrunk quality ~0.867, got %f", est.Mean)
		}
		if est.LowerCI < 0.0 || est.UpperCI > 1.0 {
			t.Errorf("Quality confidence intervals must be clamped to [0, 1]: [%f, %f]", est.LowerCI, est.UpperCI)
		}
	})

	t.Run("TestLatencyPredictionNonNegative", func(t *testing.T) {
		model := router.ModelDefinition{
			NominalP50LatencyMs: 45.0,
		}
		profile := adaptive.PerformanceProfile{
			TotalAttempts: 20,
			LatencyRegression: adaptive.LatencyRegressionState{
				Theta0: 15.0,
				Theta1: 0.01,
				Theta2: 0.02,
			},
			RecentLatencies: []float64{25.0, 30.0, 35.0, 40.0, 50.0},
		}

		est := adaptive.PredictLatency(model, profile, 500, 200)
		if est.PredictedMs <= 0.0 {
			t.Errorf("Predicted latency must be positive, got %f", est.PredictedMs)
		}
		if est.ObservedP50Ms <= 0.0 || est.ObservedP95Ms <= 0.0 {
			t.Errorf("Observed tail percentiles must be positive")
		}
	})

	t.Run("TestCostCorrectionConvergence", func(t *testing.T) {
		model := router.ModelDefinition{
			CostPer1kInputTokens:  0.001,
			CostPer1kOutputTokens: 0.002,
		}
		// Nominal sum $1.00, Actual sum $1.20 (20% higher than nominal) over 50 samples
		profile := adaptive.PerformanceProfile{
			TotalAttempts:  50,
			CostNominalSum: 1.00,
			CostActualSum:  1.20,
		}

		est := adaptive.PredictCost(model, 1000, 1000, profile)
		// Nominal cost is 0.001*1 + 0.002*1 = 0.003
		if est.NominalUSD != 0.003 {
			t.Errorf("Expected nominal cost 0.003, got %f", est.NominalUSD)
		}
		// Correction factor should be > 1.0 (converging towards 1.20)
		if est.CorrectionFactor < 1.10 || est.CorrectionFactor > 1.20 {
			t.Errorf("Expected correction factor in [1.10, 1.20], got %f", est.CorrectionFactor)
		}
	})
}

func TestAdaptive_ColdStartBayesianShrinkage(t *testing.T) {
	svc, registry, _, _, _ := setupAdaptiveTest()
	ctx := context.Background()
	models, _ := registry.ListModels(ctx)

	req := router.RoutingRequest{
		TaskID:                "task-coldstart-1",
		RunID:                 "run-cold",
		Prompt:                "summarize technical document",
		TaskComplexity:        router.ComplexityModerate,
		QualityRequirement:    0.70,
		EstimatedInputTokens:  1000,
		EstimatedOutputTokens: 500,
		SecurityProfile:       "standard",
		RoutingPolicy:         router.PolicyBalanced,
	}

	// 1. Cold start with N=0 samples: confidence = 0.0, effective utility = nominal score exactly
	decision, err := svc.Route(ctx, req)
	if err != nil {
		t.Fatalf("Routing failed: %v", err)
	}

	if decision.Confidence != 0.0 {
		t.Errorf("Expected confidence=0.0 on cold start, got %f", decision.Confidence)
	}
	if decision.SampleCount != 0 {
		t.Errorf("Expected sample count=0 on cold start, got %d", decision.SampleCount)
	}

	nomDecision, _ := router.RouteTask(req, models)
	if decision.SelectedModelID != nomDecision.SelectedModelID {
		t.Errorf("Cold start decision (%s) diverged from nominal Stage 17 decision (%s)",
			decision.SelectedModelID, nomDecision.SelectedModelID)
	}
}

func TestAdaptive_CalibratedSyntheticDriftAdaptation(t *testing.T) {
	detector := adaptive.NewDualWindowDriftDetector()

	// 1. Establish stable baseline with 100 observations on medium model (Quality=0.91, Latency=220ms)
	for i := 0; i < 100; i++ {
		detector.RecordObservation("medium-balanced-v1", 0.91, 220.0, true)
	}

	isDegraded, _ := detector.IsDegraded("medium-balanced-v1")
	if isDegraded {
		t.Fatalf("Expected model to be HEALTHY during baseline")
	}

	// 2. Inject performance degradation in recent window (Quality drops to 0.60)
	var driftFlagged bool
	var detectedReason string
	for i := 0; i < 20; i++ {
		flagged, metric, reason := detector.RecordObservation("medium-balanced-v1", 0.60, 220.0, true)
		if flagged {
			driftFlagged = true
			detectedReason = reason
			if metric != "quality_drop" {
				t.Errorf("Expected metric 'quality_drop', got '%s'", metric)
			}
		}
	}

	if !driftFlagged {
		t.Fatalf("Expected dual-window detector to flag drift under injected degradation")
	}

	isDegraded, _ = detector.IsDegraded("medium-balanced-v1")
	if !isDegraded {
		t.Fatalf("Expected model to be marked DEGRADED after quality drop, reason: %s", detectedReason)
	}
	if detector.GetDriftPenalty("medium-balanced-v1") <= 0.0 {
		t.Fatalf("Expected positive drift penalty for degraded model")
	}
}

func TestAdaptive_AppendOnlyReplayParity(t *testing.T) {
	storeA := adaptive.NewMemoryLearningStore()
	ctx := context.Background()

	req := router.RoutingRequest{
		TaskID:                "task-replay-1",
		Prompt:                "extract JSON structure",
		TaskComplexity:        router.ComplexitySimple,
		EstimatedInputTokens:  500,
		EstimatedOutputTokens: 100,
	}

	var requests []router.RoutingRequest
	for i := 0; i < 50; i++ {
		taskReq := req
		taskReq.TaskID = fmt.Sprintf("task-replay-%d", i)
		requests = append(requests, taskReq)

		outcome := router.RoutingOutcomeRecord{
			TaskID:             taskReq.TaskID,
			ModelID:            "small-fast-v1",
			ActualLatency:      35 * time.Millisecond,
			ActualCostUSD:      0.0001,
			ActualQualityScore: 0.92,
			Success:            true,
			CompletedAt:        time.Now().UTC(),
		}
		_ = storeA.RecordOutcome(ctx, outcome, taskReq)
	}

	// Export events from Store A
	events := storeA.ListEvents()
	if len(events) != 50 {
		t.Fatalf("Expected 50 events in Store A, got %d", len(events))
	}

	// Rebuild into new Store B
	storeB := adaptive.NewMemoryLearningStore()
	err := storeB.RebuildFromEvents(events, requests)
	if err != nil {
		t.Fatalf("Rebuild failed: %v", err)
	}

	key := adaptive.ComputeFeatureKey(req, "small-fast-v1")
	pA, okA := storeA.GetProfile(key)
	pB, okB := storeB.GetProfile(key)

	if !okA || !okB {
		t.Fatalf("Profile missing after rebuild: okA=%v, okB=%v", okA, okB)
	}
	if pA.TotalAttempts != pB.TotalAttempts || pA.SuccessCount != pB.SuccessCount || pA.QualitySum != pB.QualitySum {
		t.Fatalf("Replay parity failure: A=%+v, B=%+v", pA, pB)
	}
}

func TestAdaptive_ConcurrentStreamingIngestion(t *testing.T) {
	store := adaptive.NewMemoryLearningStore()
	ctx := context.Background()

	const concurrency = 100
	var wg sync.WaitGroup
	wg.Add(concurrency)

	req := router.RoutingRequest{
		Prompt:                "concurrent task execution",
		TaskComplexity:        router.ComplexityModerate,
		EstimatedInputTokens:  800,
		EstimatedOutputTokens: 200,
	}

	for i := 0; i < concurrency; i++ {
		go func(idx int) {
			defer wg.Done()
			taskReq := req
			taskReq.TaskID = fmt.Sprintf("concurrent-task-%d", idx)

			outcome := router.RoutingOutcomeRecord{
				TaskID:             taskReq.TaskID,
				ModelID:            "medium-balanced-v1",
				ActualLatency:      120 * time.Millisecond,
				ActualCostUSD:      0.001,
				ActualQualityScore: 0.88,
				Success:            true,
				CompletedAt:        time.Now().UTC(),
			}
			_ = store.RecordOutcome(ctx, outcome, taskReq)
		}(i)
	}

	wg.Wait()

	events := store.ListEvents()
	if len(events) != concurrency {
		t.Fatalf("Expected %d events recorded concurrently, got %d", concurrency, len(events))
	}
}
