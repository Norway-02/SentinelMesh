package router_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sentinelmesh/sentinelmesh/internal/outbox"
	"github.com/sentinelmesh/sentinelmesh/internal/repository/memory"
	"github.com/sentinelmesh/sentinelmesh/internal/router"
)

func setupTestRouter() (*router.Service, *router.MemoryRegistry, *router.SyntheticModelProvider, *router.MemoryDecisionRepository, *outbox.MemoryRepository) {
	registry := router.NewDefaultModelRegistry()
	provider := router.NewSyntheticModelProvider(registry, false)
	decisionRepo := router.NewMemoryDecisionRepository()
	outboxRepo := outbox.NewMemoryRepository()
	txManager := memory.NewTxManager()

	svc := router.NewService(registry, provider, decisionRepo, outboxRepo, txManager)
	return svc, registry, provider, decisionRepo, outboxRepo
}

func TestRouter_HardConstraints(t *testing.T) {
	svc, _, _, _, _ := setupTestRouter()
	ctx := context.Background()

	t.Run("Security Profile Filter", func(t *testing.T) {
		// Large model only allows ["public", "standard"], Small model allows ["airgapped"]
		req := router.RoutingRequest{
			TaskID:                "task-sec-1",
			Prompt:                "process confidential state",
			TaskComplexity:        router.ComplexityReasoningHeavy,
			EstimatedInputTokens:  500,
			EstimatedOutputTokens: 200,
			SecurityProfile:       "airgapped",
			RoutingPolicy:         router.PolicyQualityOptimized,
		}

		decision, err := svc.Route(ctx, req)
		if err != nil {
			t.Fatalf("Expected route to succeed, got error: %v", err)
		}
		// Must select small-fast-v1 because only small allows airgapped
		if decision.SelectedModelID != "small-fast-v1" {
			t.Errorf("Expected small-fast-v1 for airgapped profile, got %s", decision.SelectedModelID)
		}
	})

	t.Run("Context Capacity Filter", func(t *testing.T) {
		// 50,000 tokens exceeds small (8k) and medium (32k), only large (128k) fits
		req := router.RoutingRequest{
			TaskID:                "task-ctx-1",
			Prompt:                "long prompt",
			TaskComplexity:        router.ComplexitySimple,
			EstimatedInputTokens:  40000,
			EstimatedOutputTokens: 10000,
			SecurityProfile:       "standard",
			RoutingPolicy:         router.PolicyCostOptimized,
		}

		decision, err := svc.Route(ctx, req)
		if err != nil {
			t.Fatalf("Expected route to succeed, got error: %v", err)
		}
		if decision.SelectedModelID != "large-reasoning-v1" {
			t.Errorf("Expected large-reasoning-v1 for 50k tokens, got %s", decision.SelectedModelID)
		}
	})

	t.Run("Quality Threshold Filter", func(t *testing.T) {
		// Quality requirement 0.90 on Complex task eliminates small (0.42) and medium (0.78)
		req := router.RoutingRequest{
			TaskID:                "task-qual-1",
			Prompt:                "formal verification proof",
			TaskComplexity:        router.ComplexityComplex,
			QualityRequirement:    0.90,
			EstimatedInputTokens:  1000,
			EstimatedOutputTokens: 500,
			SecurityProfile:       "standard",
			RoutingPolicy:         router.PolicyCostOptimized,
		}

		decision, err := svc.Route(ctx, req)
		if err != nil {
			t.Fatalf("Expected route to succeed, got error: %v", err)
		}
		if decision.SelectedModelID != "large-reasoning-v1" {
			t.Errorf("Expected large-reasoning-v1 for quality >= 0.90 on complex task, got %s", decision.SelectedModelID)
		}
	})

	t.Run("Cost Budget Rejection", func(t *testing.T) {
		// Budget $0.00001 is too low for any model
		req := router.RoutingRequest{
			TaskID:                "task-cost-reject",
			Prompt:                "cheap prompt",
			TaskComplexity:        router.ComplexitySimple,
			CostBudgetUSD:         0.000001,
			EstimatedInputTokens:  500,
			EstimatedOutputTokens: 500,
			SecurityProfile:       "standard",
			RoutingPolicy:         router.PolicyCostOptimized,
		}

		_, err := svc.Route(ctx, req)
		if err == nil {
			t.Fatal("Expected error when budget cannot be met by any model, got nil")
		}
	})
}

func TestRouter_PolicyRouting(t *testing.T) {
	svc, _, _, _, _ := setupTestRouter()
	ctx := context.Background()

	t.Run("Cost Optimized Routes Simple Task to Small Model", func(t *testing.T) {
		req := router.RoutingRequest{
			TaskID:                "task-cost-1",
			Prompt:                "extract JSON fields",
			TaskComplexity:        router.ComplexitySimple,
			EstimatedInputTokens:  300,
			EstimatedOutputTokens: 100,
			SecurityProfile:       "standard",
			RoutingPolicy:         router.PolicyCostOptimized,
		}

		decision, err := svc.Route(ctx, req)
		if err != nil {
			t.Fatalf("Routing failed: %v", err)
		}
		if decision.SelectedModelID != "small-fast-v1" {
			t.Errorf("Expected small-fast-v1 for cost-optimized simple task, got %s", decision.SelectedModelID)
		}
	})

	t.Run("Quality Optimized Routes Complex Task to Large Model", func(t *testing.T) {
		req := router.RoutingRequest{
			TaskID:                "task-qual-complex",
			Prompt:                "multi-agent orchestration plan",
			TaskComplexity:        router.ComplexityReasoningHeavy,
			EstimatedInputTokens:  1000,
			EstimatedOutputTokens: 1000,
			SecurityProfile:       "standard",
			RoutingPolicy:         router.PolicyQualityOptimized,
		}

		decision, err := svc.Route(ctx, req)
		if err != nil {
			t.Fatalf("Routing failed: %v", err)
		}
		if decision.SelectedModelID != "large-reasoning-v1" {
			t.Errorf("Expected large-reasoning-v1 for quality-optimized reasoning task, got %s", decision.SelectedModelID)
		}
	})

	t.Run("Balanced Policy Evaluates Pareto Frontier", func(t *testing.T) {
		req := router.RoutingRequest{
			TaskID:                "task-balanced-1",
			Prompt:                "code refactoring review",
			TaskComplexity:        router.ComplexityModerate,
			EstimatedInputTokens:  1000,
			EstimatedOutputTokens: 500,
			SecurityProfile:       "standard",
			RoutingPolicy:         router.PolicyBalanced,
		}

		decision, err := svc.Route(ctx, req)
		if err != nil {
			t.Fatalf("Routing failed: %v", err)
		}
		if !decision.IsParetoOptimal {
			t.Errorf("Expected selected model to be on Pareto frontier")
		}
		// Medium model offers optimal balance on moderate task
		if decision.SelectedModelID != "medium-balanced-v1" {
			t.Errorf("Expected medium-balanced-v1 for balanced moderate task, got %s", decision.SelectedModelID)
		}
	})
}

func TestRouter_CircuitBreakerAndFallback(t *testing.T) {
	svc, _, provider, decisionRepo, outboxRepo := setupTestRouter()
	ctx := context.Background()

	// For a moderate task on Balanced policy, medium-balanced-v1 is selected as primary.
	// Inject 3 consecutive 429 rate limit failures on medium model
	provider.SetFault("medium-balanced-v1", router.FaultRateLimit, 3)

	req := router.RoutingRequest{
		TaskID:                "task-fallback-1",
		RunID:                 "run-123",
		AgentID:               "agent-1",
		TenantID:              "tenant-1",
		Prompt:                "translate text with nuanced context",
		TaskComplexity:        router.ComplexityModerate,
		EstimatedInputTokens:  1000,
		EstimatedOutputTokens: 500,
		SecurityProfile:       "standard",
		RoutingPolicy:         router.PolicyBalanced,
	}

	resp, err := svc.Execute(ctx, req)
	if err != nil {
		t.Fatalf("Execute failed unexpectedly: %v", err)
	}

	// Should have automatically fallen back from medium to another candidate
	if !resp.FallbackUsed {
		t.Errorf("Expected FallbackUsed=true, got false")
	}
	if resp.ModelID == "medium-balanced-v1" {
		t.Errorf("Expected fallback model execution, but ran on faulted medium model")
	}

	// Check Outbox events
	evts := outboxRepo.GetEvents()
	if len(evts) == 0 {
		t.Fatalf("Expected outbox events, got count=%d", len(evts))
	}

	hasDecided := false
	hasFallback := false
	hasCompleted := false

	for _, e := range evts {
		switch e.EventType {
		case "sentinel.router.v1.decided":
			hasDecided = true
		case "sentinel.router.v1.fallback_triggered":
			hasFallback = true
		case "sentinel.router.v1.invocation_completed":
			hasCompleted = true
		}
	}

	if !hasDecided || !hasFallback || !hasCompleted {
		t.Errorf("Expected all 3 event types (decided, fallback, completed), got decided=%v fallback=%v completed=%v", hasDecided, hasFallback, hasCompleted)
	}

	// Verify telemetry record in repository
	outcomes, _ := decisionRepo.ListOutcomes(ctx, 10)
	if len(outcomes) == 0 {
		t.Fatal("Expected outcome record in decision repository")
	}
	if !outcomes[0].FallbackUsed {
		t.Errorf("Expected outcome record to reflect FallbackUsed=true")
	}
}

func TestRouter_PolicySeparation(t *testing.T) {
	svc, _, _, _, _ := setupTestRouter()
	ctx := context.Background()

	// Base scenario where all three models meet the quality requirement (Q >= 0.70)
	// on a Moderate task:
	// Small: Qual=0.74, Cost=$0.0002, Lat=35ms
	// Medium: Qual=0.91, Cost=$0.0020, Lat=135ms
	// Large: Qual=0.97, Cost=$0.0210, Lat=475ms
	baseReq := router.RoutingRequest{
		TaskID:                "task-separation",
		RunID:                 "run-sep",
		Prompt:                "summarize technical document",
		TaskComplexity:        router.ComplexityModerate,
		QualityRequirement:    0.70,
		EstimatedInputTokens:  1000,
		EstimatedOutputTokens: 500,
		SecurityProfile:       "standard",
	}

	t.Run("Cost Policy Selects Cheapest Feasible Model", func(t *testing.T) {
		req := baseReq
		req.RoutingPolicy = router.PolicyCostOptimized
		decision, err := svc.Route(ctx, req)
		if err != nil {
			t.Fatalf("Routing failed: %v", err)
		}
		if decision.SelectedModelID != "small-fast-v1" {
			t.Errorf("Expected small-fast-v1 for cost policy, got %s", decision.SelectedModelID)
		}
	})

	t.Run("Quality Policy Selects Highest Quality Model", func(t *testing.T) {
		req := baseReq
		req.RoutingPolicy = router.PolicyQualityOptimized
		decision, err := svc.Route(ctx, req)
		if err != nil {
			t.Fatalf("Routing failed: %v", err)
		}
		if decision.SelectedModelID != "large-reasoning-v1" {
			t.Errorf("Expected large-reasoning-v1 for quality policy, got %s", decision.SelectedModelID)
		}
	})

	t.Run("Balanced Policy Selects Pareto Tradeoff Model", func(t *testing.T) {
		req := baseReq
		req.RoutingPolicy = router.PolicyBalanced
		decision, err := svc.Route(ctx, req)
		if err != nil {
			t.Fatalf("Routing failed: %v", err)
		}
		if decision.SelectedModelID != "medium-balanced-v1" {
			t.Errorf("Expected medium-balanced-v1 for balanced policy, got %s", decision.SelectedModelID)
		}
	})
}

func TestRouter_DecisionExplainability(t *testing.T) {
	svc, _, _, decisionRepo, _ := setupTestRouter()
	ctx := context.Background()

	req := router.RoutingRequest{
		TaskID:                "task-explain-1",
		RunID:                 "run-explain-1",
		Prompt:                "complex formal proof",
		TaskComplexity:        router.ComplexityComplex,
		QualityRequirement:    0.90, // Rejects small (0.42) and medium (0.78)
		EstimatedInputTokens:  1000,
		EstimatedOutputTokens: 500,
		SecurityProfile:       "standard",
		RoutingPolicy:         router.PolicyBalanced,
	}

	decision, err := svc.Route(ctx, req)
	if err != nil {
		t.Fatalf("Routing failed: %v", err)
	}

	if decision.SelectedModelID != "large-reasoning-v1" {
		t.Fatalf("Expected large-reasoning-v1, got %s", decision.SelectedModelID)
	}

	if len(decision.Rejections) != 2 {
		t.Fatalf("Expected 2 rejections, got %d", len(decision.Rejections))
	}

	for _, r := range decision.Rejections {
		if r.Reason != "quality_below_threshold" {
			t.Errorf("Expected reason 'quality_below_threshold' for rejected model %s, got '%s'", r.ModelID, r.Reason)
		}
		if r.Details == "" {
			t.Errorf("Expected non-empty details for rejected model %s", r.ModelID)
		}
	}

	// Verify persistence of explainability fields
	persisted, err := decisionRepo.GetDecision(ctx, req.TaskID)
	if err != nil {
		t.Fatalf("Failed to retrieve persisted decision: %v", err)
	}
	if len(persisted.Rejections) != 2 {
		t.Errorf("Persisted decision missing rejections, got %d", len(persisted.Rejections))
	}
	if persisted.RegistryVersion == "" || persisted.PolicyVersion == "" {
		t.Errorf("Expected versioning metadata on persisted decision, got reg='%s', pol='%s'",
			persisted.RegistryVersion, persisted.PolicyVersion)
	}
}

func TestRouter_DeterministicReplay(t *testing.T) {
	registry := router.NewDefaultModelRegistry()
	ctx := context.Background()
	models, _ := registry.ListModels(ctx)

	req := router.RoutingRequest{
		TaskID:                "task-replay-1000",
		RunID:                 "run-replay",
		Prompt:                "deterministic replay verification test",
		TaskComplexity:        router.ComplexityModerate,
		QualityRequirement:    0.75,
		EstimatedInputTokens:  800,
		EstimatedOutputTokens: 250,
		SecurityProfile:       "standard",
		RoutingPolicy:         router.PolicyBalanced,
	}

	original, err := router.RouteTask(req, models)
	if err != nil {
		t.Fatalf("Initial RouteTask failed: %v", err)
	}

	// Replay 1,000 times to verify absolute zero-drift determinism
	for i := 0; i < 1000; i++ {
		replayed, err := router.Replay(req, models)
		if err != nil {
			t.Fatalf("Replay iteration %d failed: %v", i, err)
		}

		if replayed.SelectedModelID != original.SelectedModelID {
			t.Fatalf("Replay drift on model ID: original=%s, replayed=%s", original.SelectedModelID, replayed.SelectedModelID)
		}
		if replayed.FinalScore != original.FinalScore {
			t.Fatalf("Replay drift on score: original=%f, replayed=%f", original.FinalScore, replayed.FinalScore)
		}
		if replayed.ScoreBreakdown != original.ScoreBreakdown {
			t.Fatalf("Replay drift on score breakdown")
		}
		if len(replayed.FallbackCandidates) != len(original.FallbackCandidates) {
			t.Fatalf("Replay drift on fallback count")
		}
	}
}

func TestRouter_CircuitBreaker_ErrorFilteringAndProbing(t *testing.T) {
	cb := router.NewCircuitBreaker("test-model", 3, 50*time.Millisecond)

	// 1. ClientError (e.g. 400 Bad Request) must NOT increment failures or trip breaker
	clientErr := &router.ClientError{Code: "BAD_REQUEST", Message: "Invalid prompt schema"}
	for i := 0; i < 5; i++ {
		cb.RecordFailure(clientErr)
	}
	if cb.State() != router.StateClosed {
		t.Fatalf("Expected StateClosed after client errors, got %s", cb.State())
	}

	// 2. InfrastructureError (e.g. 503, 429, timeout) MUST trip breaker after 3 failures
	infraErr := &router.InfrastructureError{Code: "HTTP_429", Message: "Rate limit exceeded"}
	cb.RecordFailure(infraErr)
	cb.RecordFailure(infraErr)
	if cb.State() != router.StateClosed {
		t.Fatalf("Expected StateClosed after 2 infra failures, got %s", cb.State())
	}
	cb.RecordFailure(infraErr)
	if cb.State() != router.StateOpen {
		t.Fatalf("Expected StateOpen after 3 infra failures, got %s", cb.State())
	}

	// While Open and within cooldown, requests are denied
	if cb.AllowExecution() {
		t.Fatalf("Expected AllowExecution=false while Open within cooldown")
	}

	// Wait for cooldown
	time.Sleep(60 * time.Millisecond)

	// In HalfOpen, 1st probe is allowed
	if !cb.AllowExecution() {
		t.Fatalf("Expected 1st probe in HalfOpen to be allowed")
	}
	// Concurrent 2nd probe must be denied while probe is in flight
	if cb.AllowExecution() {
		t.Fatalf("Expected concurrent 2nd probe in HalfOpen to be blocked")
	}

	// Probe succeeds twice -> transitions back to Closed
	cb.RecordSuccess()
	cb.RecordSuccess()
	if cb.State() != router.StateClosed {
		t.Fatalf("Expected StateClosed after successful probes, got %s", cb.State())
	}
}

func TestRouter_CascadingMultiStepFallback(t *testing.T) {
	svc, _, provider, decisionRepo, _ := setupTestRouter()
	ctx := context.Background()

	// Setup task that prefers Large (ReasoningHeavy + QualityOptimized)
	// Fallback candidates will be Medium then Small.
	// Inject faults on both Large and Medium!
	provider.SetFault("large-reasoning-v1", router.FaultTimeout, 3)
	provider.SetFault("medium-balanced-v1", router.FaultServerError, 3)

	req := router.RoutingRequest{
		TaskID:                "task-cascading-fallback",
		RunID:                 "run-cascade",
		AgentID:               "agent-cascade",
		TenantID:              "tenant-cascade",
		Prompt:                "multi-agent consensus verification",
		TaskComplexity:        router.ComplexityReasoningHeavy,
		QualityRequirement:    0.10, // Relaxed so small is feasible as last resort
		EstimatedInputTokens:  1000,
		EstimatedOutputTokens: 500,
		SecurityProfile:       "standard",
		RoutingPolicy:         router.PolicyQualityOptimized,
	}

	resp, err := svc.Execute(ctx, req)
	if err != nil {
		t.Fatalf("Execution failed: %v", err)
	}

	// Should have cascaded through Large (failed) -> Medium (failed) -> Small (succeeded)
	if resp.ModelID != "small-fast-v1" {
		t.Fatalf("Expected execution on tertiary model 'small-fast-v1', got '%s'", resp.ModelID)
	}
	if !resp.FallbackUsed {
		t.Errorf("Expected FallbackUsed=true")
	}
	if resp.AttemptNumber != 3 {
		t.Errorf("Expected AttemptNumber=3, got %d", resp.AttemptNumber)
	}

	outcomes, _ := decisionRepo.ListOutcomes(ctx, 10)
	if len(outcomes) == 0 {
		t.Fatal("Expected outcome record in decision repo")
	}
	if outcomes[0].AttemptNumber != 3 || !outcomes[0].FallbackUsed {
		t.Errorf("Persisted outcome did not reflect 3rd attempt fallback: attempt=%d fallback=%v",
			outcomes[0].AttemptNumber, outcomes[0].FallbackUsed)
	}
}

func TestRouter_Policy4WayDivergence(t *testing.T) {
	registry := router.NewDefaultModelRegistry()
	ctx := context.Background()

	// Register a 4th specialized model: "batch-cheap-v1" which is cheaper than small-fast-v1 but slower
	_ = registry.RegisterModel(ctx, router.ModelDefinition{
		ID:                    "batch-cheap-v1",
		Name:                  "Batch Cheap Model",
		Tier:                  router.TierSmall,
		Provider:              "synthetic-batch",
		CostPer1kInputTokens:  0.00005, // 3x cheaper than small-fast-v1 ($0.00015)
		CostPer1kOutputTokens: 0.00020,
		NominalP50LatencyMs:   250.0,   // slower than small-fast-v1 (45ms)
		BaseOverheadMs:        100.0,
		InputMsPer1kTokens:    50.0,
		OutputMsPer1kTokens:   150.0,
		ContextWindow:         16384,
		SecurityClasses:       []string{"public", "standard", "restricted"},
		HealthStatus:          router.HealthHealthy,
		TaskQualityMatrix: map[router.TaskComplexity]float64{
			router.ComplexitySimple:         0.90,
			router.ComplexityModerate:       0.80,
			router.ComplexityComplex:        0.50,
			router.ComplexityReasoningHeavy: 0.20,
		},
	})

	models, _ := registry.ListModels(ctx)

	// Workload: Moderate task with QualityRequirement = 0.70
	// 1. batch-cheap-v1:   Cost=$0.00015, Lat=300ms, Qual=0.80
	// 2. small-fast-v1:    Cost=$0.00045, Lat=42.5ms, Qual=0.74
	// 3. medium-balanced:  Cost=$0.00450, Lat=170ms, Qual=0.91
	// 4. large-reasoning:  Cost=$0.04500, Lat=655ms, Qual=0.97
	req := router.RoutingRequest{
		TaskID:                "task-4way-div",
		RunID:                 "run-div",
		Prompt:                "process moderate data transformation",
		TaskComplexity:        router.ComplexityModerate,
		QualityRequirement:    0.70,
		EstimatedInputTokens:  1000,
		EstimatedOutputTokens: 500,
		SecurityProfile:       "standard",
	}

	// 1. Cost policy MUST choose batch-cheap-v1
	costReq := req
	costReq.RoutingPolicy = router.PolicyCostOptimized
	dCost, err := router.RouteTask(costReq, models)
	if err != nil || dCost.SelectedModelID != "batch-cheap-v1" {
		t.Fatalf("Expected batch-cheap-v1 for Cost policy, got %s (err: %v)", dCost.SelectedModelID, err)
	}

	// 2. Latency policy MUST choose small-fast-v1
	latReq := req
	latReq.RoutingPolicy = router.PolicyLatencyOptimized
	dLat, err := router.RouteTask(latReq, models)
	if err != nil || dLat.SelectedModelID != "small-fast-v1" {
		t.Fatalf("Expected small-fast-v1 for Latency policy, got %s (err: %v)", dLat.SelectedModelID, err)
	}

	// 3. Quality policy MUST choose large-reasoning-v1
	qualReq := req
	qualReq.RoutingPolicy = router.PolicyQualityOptimized
	dQual, err := router.RouteTask(qualReq, models)
	if err != nil || dQual.SelectedModelID != "large-reasoning-v1" {
		t.Fatalf("Expected large-reasoning-v1 for Quality policy, got %s (err: %v)", dQual.SelectedModelID, err)
	}

	// 4. Balanced policy MUST choose medium-balanced-v1
	balReq := req
	balReq.RoutingPolicy = router.PolicyBalanced
	dBal, err := router.RouteTask(balReq, models)
	if err != nil || dBal.SelectedModelID != "medium-balanced-v1" {
		t.Fatalf("Expected medium-balanced-v1 for Balanced policy, got %s (err: %v)", dBal.SelectedModelID, err)
	}

	// Assert 4-way distinct model selections
	selections := map[string]string{
		"cost":     dCost.SelectedModelID,
		"latency":  dLat.SelectedModelID,
		"quality":  dQual.SelectedModelID,
		"balanced": dBal.SelectedModelID,
	}

	uniqueModels := make(map[string]bool)
	for _, m := range selections {
		uniqueModels[m] = true
	}
	if len(uniqueModels) != 4 {
		t.Fatalf("Expected 4 distinct models across 4 policies, got %d unique: %v", len(uniqueModels), selections)
	}
}

func TestRouter_ReplayAfterRegistryChange(t *testing.T) {
	registry := router.NewDefaultModelRegistry()
	ctx := context.Background()

	// 1. Take Snapshot S0 at t=0
	s0, _ := registry.Snapshot(ctx)

	req := router.RoutingRequest{
		TaskID:                "task-snapshot-replay",
		RunID:                 "run-snap",
		Prompt:                "complex code generation",
		TaskComplexity:        router.ComplexityModerate,
		QualityRequirement:    0.70,
		EstimatedInputTokens:  1000,
		EstimatedOutputTokens: 500,
		SecurityProfile:       "standard",
		RoutingPolicy:         router.PolicyBalanced,
		RegistryVersion:       registry.Version(),
	}

	// Initial decision under S0
	d0, err := router.RouteTask(req, s0)
	if err != nil {
		t.Fatalf("Initial decision failed: %v", err)
	}
	if d0.SelectedModelID != "medium-balanced-v1" {
		t.Fatalf("Expected medium-balanced-v1 under S0, got %s", d0.SelectedModelID)
	}

	// 2. Modify registry live at t=1 (e.g. Medium model price increases 100x and latency increases 10x)
	medModel, _ := registry.GetModel(ctx, "medium-balanced-v1")
	medModel.CostPer1kInputTokens = 0.50
	medModel.CostPer1kOutputTokens = 2.00
	medModel.NominalP50LatencyMs = 5000.0
	_ = registry.RegisterModel(ctx, medModel)

	s1, _ := registry.Snapshot(ctx)

	// Decision under updated S1 should now choose a different model
	d1, err := router.RouteTask(req, s1)
	if err != nil {
		t.Fatalf("Decision under S1 failed: %v", err)
	}
	if d1.SelectedModelID == "medium-balanced-v1" {
		t.Fatalf("Expected S1 to reject expensive medium model, but selected it")
	}

	// 3. Historical Replay using stored S0 snapshot MUST reproduce the exact initial decision d0
	dReplay, err := router.Replay(req, s0)
	if err != nil {
		t.Fatalf("Historical replay failed: %v", err)
	}
	if dReplay.SelectedModelID != d0.SelectedModelID {
		t.Fatalf("Replay divergence: expected %s, got %s", d0.SelectedModelID, dReplay.SelectedModelID)
	}
	if dReplay.FinalScore != d0.FinalScore {
		t.Fatalf("Replay score divergence: expected %f, got %f", d0.FinalScore, dReplay.FinalScore)
	}
}

func TestRouter_ConcurrentHalfOpenProbe(t *testing.T) {
	cb := router.NewCircuitBreaker("test-concurrent-model", 3, 50*time.Millisecond)

	// Trip breaker to Open
	infraErr := &router.InfrastructureError{Code: "HTTP_503", Message: "Unavailable"}
	cb.RecordFailure(infraErr)
	cb.RecordFailure(infraErr)
	cb.RecordFailure(infraErr)
	if cb.State() != router.StateOpen {
		t.Fatalf("Expected StateOpen, got %s", cb.State())
	}

	// Wait for cooldown to expire so breaker can transition to HalfOpen
	time.Sleep(60 * time.Millisecond)

	// 100 concurrent requests all try to execute through the half-open breaker
	const concurrency = 100
	var wg sync.WaitGroup
	wg.Add(concurrency)

	var allowedCount int64
	var deniedCount int64

	for i := 0; i < concurrency; i++ {
		go func() {
			defer wg.Done()
			if cb.AllowExecution() {
				atomic.AddInt64(&allowedCount, 1)
			} else {
				atomic.AddInt64(&deniedCount, 1)
			}
		}()
	}

	wg.Wait()

	// Exactly ONE request must have been granted probe status
	if allowedCount != 1 {
		t.Fatalf("Expected exactly 1 probe allowed in HalfOpen state, got %d (denied: %d)", allowedCount, deniedCount)
	}
	if deniedCount != concurrency-1 {
		t.Fatalf("Expected %d requests denied, got %d", concurrency-1, deniedCount)
	}
}
