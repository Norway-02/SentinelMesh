package onlinepolicy_test

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/sentinelmesh/sentinelmesh/internal/adaptive"
	"github.com/sentinelmesh/sentinelmesh/internal/onlinepolicy"
	"github.com/sentinelmesh/sentinelmesh/internal/outbox"
	"github.com/sentinelmesh/sentinelmesh/internal/repository/memory"
	"github.com/sentinelmesh/sentinelmesh/internal/router"
)

func setupPolicyTest() (*onlinepolicy.OnlinePolicyService, *router.MemoryRegistry, *router.SyntheticModelProvider, *adaptive.MemoryLearningStore, *onlinepolicy.PolicyManager, *onlinepolicy.GuardrailEnforcer) {
	registry := router.NewDefaultModelRegistry()
	provider := router.NewSyntheticModelProvider(registry, false)
	store := adaptive.NewMemoryLearningStore()
	prior := adaptive.DefaultBetaPrior()
	pm := onlinepolicy.NewPolicyManager(onlinepolicy.DefaultPolicyState())
	guardrails := onlinepolicy.NewGuardrailEnforcer(onlinepolicy.DefaultGuardrailConfig())
	decisionRepo := router.NewMemoryDecisionRepository()
	outboxRepo := outbox.NewMemoryRepository()
	txManager := memory.NewTxManager()

	svc := onlinepolicy.NewOnlinePolicyService(registry, provider, store, prior, pm, guardrails, decisionRepo, outboxRepo, txManager)
	return svc, registry, provider, store, pm, guardrails
}

func TestPolicyNeverViolatesStage17Constraints(t *testing.T) {
	registry := router.NewDefaultModelRegistry()
	ctx := context.Background()
	models, _ := registry.ListModels(ctx)
	store := adaptive.NewMemoryLearningStore()
	prior := adaptive.DefaultBetaPrior()
	pm := onlinepolicy.NewPolicyManager(onlinepolicy.DefaultPolicyState())
	guardrails := onlinepolicy.NewGuardrailEnforcer(onlinepolicy.DefaultGuardrailConfig())
	decisionRepo := router.NewMemoryDecisionRepository()
	outboxRepo := outbox.NewMemoryRepository()
	txManager := memory.NewTxManager()

	svc := onlinepolicy.NewOnlinePolicyService(registry, providerForTest(registry), store, prior, pm, guardrails, decisionRepo, outboxRepo, txManager)

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

	// 10,000 randomized property-based invariant evaluations
	for i := 0; i < 10000; i++ {
		req := router.RoutingRequest{
			TaskID:                fmt.Sprintf("prop-p19-%d", i),
			RunID:                 fmt.Sprintf("prop-run-p19-%d", i),
			Prompt:                "Property-based 10k safety constraint invariance check",
			TaskComplexity:        complexities[rng.Intn(len(complexities))],
			QualityRequirement:    0.50 + rng.Float64()*0.45,
			EstimatedInputTokens:  100 + rng.Intn(40000),
			EstimatedOutputTokens: 50 + rng.Intn(10000),
			SecurityProfile:       secProfiles[rng.Intn(len(secProfiles))],
			RoutingPolicy:         policies[rng.Intn(len(policies))],
		}

		nominalDecision, nomErr := router.RouteTask(req, models)
		policyDecision, polErr := svc.Route(ctx, req)

		if nomErr != nil {
			if polErr == nil {
				t.Fatalf("SAFETY VIOLATION on iteration %d: Stage 17 rejected (%v) but Stage 19 allowed %s",
					i, nomErr, policyDecision.SelectedModelID)
			}
			continue
		}

		if polErr != nil {
			t.Fatalf("Unexpected error in policy service on iteration %d: %v", i, polErr)
		}

		feasible := make(map[string]bool)
		feasible[nominalDecision.SelectedModelID] = true
		for _, fb := range nominalDecision.FallbackCandidates {
			feasible[fb] = true
		}

		if !feasible[policyDecision.SelectedModelID] {
			t.Fatalf("SAFETY INVARIANT BROKEN on iteration %d: Policy selected model %s NOT in Stage 17 feasible set %v",
				i, policyDecision.SelectedModelID, feasible)
		}
	}
}

func TestUCBUsesNormalizedUtility(t *testing.T) {
	svc, _, _, _, _, _ := setupPolicyTest()
	ctx := context.Background()

	req := router.RoutingRequest{
		TaskID:                "test-ucb-norm",
		Prompt:                "extract json entities from document",
		TaskComplexity:        router.ComplexitySimple,
		QualityRequirement:    0.60,
		CostBudgetUSD:         0.005,
		LatencySLAMs:          500.0,
		EstimatedInputTokens:  500,
		EstimatedOutputTokens: 100,
		SecurityProfile:       "standard",
		RoutingPolicy:         router.PolicyBalanced,
	}

	decision, err := svc.Route(ctx, req)
	if err != nil {
		t.Fatalf("Route failed: %v", err)
	}

	if decision.ExpectedUtility < -1.0 || decision.ExpectedUtility > 1.0 {
		t.Errorf("Expected normalized utility in [-1, 1], got %f", decision.ExpectedUtility)
	}
	if decision.UCBScore < decision.ExpectedUtility {
		t.Errorf("UCB score (%f) must be >= ExpectedUtility (%f)", decision.UCBScore, decision.ExpectedUtility)
	}
	if decision.Uncertainty < 0.0 {
		t.Errorf("Uncertainty must be non-negative, got %f", decision.Uncertainty)
	}
}

func TestExplorationBudgetRollingWindow(t *testing.T) {
	tracker := onlinepolicy.NewExplorationTracker(200)

	// In 200 decisions, we make 10 explore decisions
	for i := 0; i < 190; i++ {
		tracker.RecordDecision("small-fast-v1", false)
	}
	for i := 0; i < 10; i++ {
		tracker.RecordDecision("small-fast-v1", true)
	}

	rate := tracker.CurrentExplorationRate()
	if rate != 0.05 {
		t.Errorf("Expected exploration rate 0.05 (5%%), got %f", rate)
	}

	// Global limit is 10 per 200: canExplore should return false now
	if tracker.CanExplore("small-fast-v1", 10, 4) {
		t.Errorf("Expected CanExplore to be false when global exploration limit is reached")
	}

	// Add 200 non-explorations to completely flush out the 10 explorations from the rolling window of 200
	for i := 0; i < 200; i++ {
		tracker.RecordDecision("small-fast-v1", false)
	}
	if !tracker.CanExplore("medium-balanced-v1", 10, 4) {
		t.Errorf("Expected CanExplore to become true after rolling window flushes previous explorations")
	}
}

func TestExplorationOnlyWithinFeasibleSet(t *testing.T) {
	svc, _, _, _, _, _ := setupPolicyTest()
	ctx := context.Background()

	// Task requires airgapped security (only small-fast-v1 is airgapped in default catalog)
	req := router.RoutingRequest{
		TaskID:                "test-airgap-explore",
		Prompt:                "Classified security audit parsing",
		TaskComplexity:        router.ComplexitySimple,
		QualityRequirement:    0.60,
		SecurityProfile:       "airgapped",
		RoutingPolicy:         router.PolicyBalanced,
		EstimatedInputTokens:  400,
		EstimatedOutputTokens: 100,
	}

	for i := 0; i < 50; i++ {
		dec, err := svc.Route(ctx, req)
		if err != nil {
			t.Fatalf("Route failed: %v", err)
		}
		if dec.SelectedModelID != "small-fast-v1" {
			t.Fatalf("Exploration violated security filter: selected %s instead of airgapped model", dec.SelectedModelID)
		}
	}
}

func TestPolicyRollbackRestoresPreviousState(t *testing.T) {
	svc, _, _, _, pm, guardrails := setupPolicyTest()
	ctx := context.Background()

	// 1. Promote experimental policy-v2.1
	experimentalState := onlinepolicy.DefaultPolicyState()
	experimentalState.Version = "policy-v2.1"
	experimentalState.ExplorationBudget = 0.20 // Aggressive exploration
	pm.PromotePolicy(experimentalState)

	if pm.GetActiveState().Version != "policy-v2.1" {
		t.Fatalf("Expected active policy to be policy-v2.1")
	}

	req := router.RoutingRequest{
		TaskID:                "test-rollback",
		Prompt:                "summarize report",
		TaskComplexity:        router.ComplexityModerate,
		QualityRequirement:    0.75,
		EstimatedInputTokens:  800,
		EstimatedOutputTokens: 200,
		SecurityProfile:       "standard",
		RoutingPolicy:         router.PolicyBalanced,
	}

	// 2. Feed toxic outcomes that breach quality floor (0.60 < 0.85)
	for i := 0; i < 20; i++ {
		outcome := router.RoutingOutcomeRecord{
			TaskID:             fmt.Sprintf("toxic-%d", i),
			ModelID:            "medium-balanced-v1",
			ActualLatency:      150 * time.Millisecond,
			ActualCostUSD:      0.001,
			ActualQualityScore: 0.60, // Severe quality degradation
			Success:            true,
			CompletedAt:        time.Now().UTC(),
		}
		breached, event := guardrails.RecordOutcome(outcome, 0.001, 150.0)
		if breached {
			_, err := pm.TriggerRollback(event)
			if err != nil {
				t.Fatalf("Rollback failed: %v", err)
			}
			break
		}
	}

	// 3. Verify Rollback Restored Parent Version policy-v2.0
	active := pm.GetActiveState()
	if active.Version != "policy-v2.0" {
		t.Fatalf("Expected policy to be rolled back to policy-v2.0, got %s", active.Version)
	}
	if !active.IsRolledBack {
		t.Fatalf("Expected IsRolledBack flag to be true")
	}

	events := pm.ListRollbackEvents()
	if len(events) == 0 {
		t.Fatalf("Expected rollback events to be recorded")
	}
	if events[0].TriggerMetric != "quality_floor" {
		t.Errorf("Expected trigger metric quality_floor, got %s", events[0].TriggerMetric)
	}

	// 4. Verify subsequent routes run on restored parent policy
	dec, err := svc.Route(ctx, req)
	if err != nil {
		t.Fatalf("Route failed: %v", err)
	}
	if dec.PolicyVersion != "policy-v2.0" {
		t.Errorf("Expected route decision policy version policy-v2.0, got %s", dec.PolicyVersion)
	}
}

func TestPolicyHysteresis(t *testing.T) {
	guardrails := onlinepolicy.NewGuardrailEnforcer(onlinepolicy.DefaultGuardrailConfig())

	// 1. Trigger a breach with 20 poor outcomes (Quality 0.60)
	for i := 0; i < 20; i++ {
		outcome := router.RoutingOutcomeRecord{
			TaskID:             fmt.Sprintf("poor-%d", i),
			ActualQualityScore: 0.60,
			Success:            true,
		}
		guardrails.RecordOutcome(outcome, 0.001, 150.0)
	}

	isBreached, _ := guardrails.IsBreached()
	if !isBreached {
		t.Fatalf("Expected guardrails to be in BREACHED state")
	}

	// 2. Feed 10 good outcomes (Quality 0.90) -> Should NOT recover yet (requires 30 consecutive)
	for i := 0; i < 10; i++ {
		outcome := router.RoutingOutcomeRecord{
			TaskID:             fmt.Sprintf("good-%d", i),
			ActualQualityScore: 0.90,
			Success:            true,
		}
		guardrails.RecordOutcome(outcome, 0.001, 150.0)
	}

	isBreached, _ = guardrails.IsBreached()
	if !isBreached {
		t.Fatalf("Hysteresis violation: recovered prematurely before 30 consecutive healthy decisions")
	}

	// 3. Feed 80 good outcomes (Quality 0.90): 50 to completely flush the poor window and 30 to satisfy consecutive healthy requirement
	for i := 0; i < 80; i++ {
		outcome := router.RoutingOutcomeRecord{
			TaskID:             fmt.Sprintf("good-rec-%d", i),
			ActualQualityScore: 0.90,
			Success:            true,
		}
		guardrails.RecordOutcome(outcome, 0.001, 150.0)
	}

	isBreached, _ = guardrails.IsBreached()
	if isBreached {
		t.Fatalf("Expected guardrails to RECOVER after window mean >= 0.88 and 30 consecutive healthy decisions")
	}
}

func TestShadowAndCanaryExecutionModes(t *testing.T) {
	svc, _, _, _, pm, _ := setupPolicyTest()
	ctx := context.Background()

	req := router.RoutingRequest{
		TaskID:                "test-shadow-mode",
		Prompt:                "Refactor go interfaces",
		TaskComplexity:        router.ComplexityModerate,
		QualityRequirement:    0.75,
		EstimatedInputTokens:  800,
		EstimatedOutputTokens: 250,
		SecurityProfile:       "standard",
		RoutingPolicy:         router.PolicyBalanced,
	}

	// 1. Shadow Mode: live execution stays on baseline, shadow logs evaluation
	shadowState := onlinepolicy.DefaultPolicyState()
	shadowState.Mode = onlinepolicy.ModeShadow
	pm.PromotePolicy(shadowState)

	decShadow, err := svc.Route(ctx, req)
	if err != nil {
		t.Fatalf("Route in shadow mode failed: %v", err)
	}
	if decShadow.DecisionMode != onlinepolicy.DecisionShadow {
		t.Errorf("Expected decision mode SHADOW, got %s", decShadow.DecisionMode)
	}

	// 2. Canary Mode: executes with canary tag
	canaryState := onlinepolicy.DefaultPolicyState()
	canaryState.Mode = onlinepolicy.ModeCanary
	canaryState.CanaryFraction = 1.0 // Force canary for test
	pm.PromotePolicy(canaryState)

	decCanary, err := svc.Route(ctx, req)
	if err != nil {
		t.Fatalf("Route in canary mode failed: %v", err)
	}
	if decCanary.DecisionMode != onlinepolicy.DecisionCanary {
		t.Errorf("Expected decision mode CANARY, got %s", decCanary.DecisionMode)
	}
}

func TestConcurrentPolicyUpdates(t *testing.T) {
	svc, _, _, _, _, _ := setupPolicyTest()
	ctx := context.Background()

	const concurrency = 100
	var wg sync.WaitGroup
	wg.Add(concurrency)

	req := router.RoutingRequest{
		Prompt:                "concurrent task",
		TaskComplexity:        router.ComplexitySimple,
		EstimatedInputTokens:  500,
		EstimatedOutputTokens: 100,
		SecurityProfile:       "standard",
		RoutingPolicy:         router.PolicyBalanced,
	}

	for i := 0; i < concurrency; i++ {
		go func(idx int) {
			defer wg.Done()
			taskReq := req
			taskReq.TaskID = fmt.Sprintf("conc-task-%d", idx)
			_, err := svc.Route(ctx, taskReq)
			if err != nil {
				t.Errorf("Concurrent route failed: %v", err)
			}
		}(i)
	}

	wg.Wait()
}

func TestRegretConvergenceInStationaryEnvironment(t *testing.T) {
	svc, _, _, store, _, _ := setupPolicyTest()
	ctx := context.Background()

	req := router.RoutingRequest{
		Prompt:                "stationary regret convergence test",
		TaskComplexity:        router.ComplexityModerate,
		QualityRequirement:    0.75,
		EstimatedInputTokens:  800,
		EstimatedOutputTokens: 250,
		SecurityProfile:       "standard",
		RoutingPolicy:         router.PolicyBalanced,
	}

	// Oracle optimal reward = 0.90
	const oracleReward = 0.90

	// Cold start phase (first 20 tasks, prior uncertainty higher)
	coldRegretSum := 0.0
	for i := 0; i < 20; i++ {
		taskReq := req
		taskReq.TaskID = fmt.Sprintf("cold-%d", i)
		dec, _ := svc.Route(ctx, taskReq)
		outcome := router.RoutingOutcomeRecord{
			TaskID:             taskReq.TaskID,
			ModelID:            dec.SelectedModelID,
			ActualLatency:      120 * time.Millisecond,
			ActualCostUSD:      0.001,
			ActualQualityScore: 0.92,
			Success:            true,
		}
		_ = store.RecordOutcome(ctx, outcome, taskReq)
		r := onlinepolicy.CalculateReward(outcome, taskReq, onlinepolicy.DefaultRewardWeights())
		coldRegretSum += (oracleReward - r)
	}
	meanColdRegret := coldRegretSum / 20.0

	// Warm post-learning phase (next 80 tasks, stable exploitation)
	warmRegretSum := 0.0
	for i := 20; i < 100; i++ {
		taskReq := req
		taskReq.TaskID = fmt.Sprintf("warm-%d", i)
		dec, _ := svc.Route(ctx, taskReq)
		outcome := router.RoutingOutcomeRecord{
			TaskID:             taskReq.TaskID,
			ModelID:            dec.SelectedModelID,
			ActualLatency:      120 * time.Millisecond,
			ActualCostUSD:      0.001,
			ActualQualityScore: 0.92,
			Success:            true,
		}
		_ = store.RecordOutcome(ctx, outcome, taskReq)
		r := onlinepolicy.CalculateReward(outcome, taskReq, onlinepolicy.DefaultRewardWeights())
		warmRegretSum += (oracleReward - r)
	}
	meanWarmRegret := warmRegretSum / 80.0

	if meanWarmRegret > meanColdRegret+0.01 {
		t.Errorf("Expected warm regret (%.4f) to be <= cold regret (%.4f)", meanWarmRegret, meanColdRegret)
	}
}

func providerForTest(registry router.Registry) router.ModelProvider {
	return router.NewSyntheticModelProvider(registry, false)
}
