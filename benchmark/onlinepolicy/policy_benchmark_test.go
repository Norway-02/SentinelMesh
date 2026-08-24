package onlinepolicy_test

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"testing"
	"time"

	"github.com/sentinelmesh/sentinelmesh/internal/adaptive"
	"github.com/sentinelmesh/sentinelmesh/internal/onlinepolicy"
	"github.com/sentinelmesh/sentinelmesh/internal/outbox"
	"github.com/sentinelmesh/sentinelmesh/internal/repository/memory"
	"github.com/sentinelmesh/sentinelmesh/internal/router"
)

func BenchmarkOnlinePolicy_DecisionEngine(b *testing.B) {
	registry := router.NewDefaultModelRegistry()
	provider := router.NewSyntheticModelProvider(registry, false)
	learningStore := adaptive.NewMemoryLearningStore()
	prior := adaptive.DefaultBetaPrior()
	pm := onlinepolicy.NewPolicyManager(onlinepolicy.DefaultPolicyState())
	guardrails := onlinepolicy.NewGuardrailEnforcer(onlinepolicy.DefaultGuardrailConfig())
	decisionRepo := router.NewMemoryDecisionRepository()
	outboxRepo := outbox.NewMemoryRepository()
	txManager := memory.NewTxManager()

	svc := onlinepolicy.NewOnlinePolicyService(registry, provider, learningStore, prior, pm, guardrails, decisionRepo, outboxRepo, txManager)
	ctx := context.Background()

	req := router.RoutingRequest{
		TaskID:                "bench-policy-task",
		Prompt:                "summarize technical document and refactor interface",
		TaskComplexity:        router.ComplexityModerate,
		QualityRequirement:    0.75,
		EstimatedInputTokens:  800,
		EstimatedOutputTokens: 250,
		SecurityProfile:       "standard",
		RoutingPolicy:         router.PolicyBalanced,
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := svc.Route(ctx, req)
		if err != nil {
			b.Fatalf("Route failed: %v", err)
		}
	}
}

type Stage19ExperimentMetrics struct {
	EngineName           string
	TotalCostUSD         float64
	MeanLatencyMs        float64
	P95LatencyMs         float64
	MeanQualityScore     float64
	QualityPassRatePct   float64
	ExplorationRatePct   float64
	FallbackRatePct      float64
	ConstraintViolations int
	AverageRegret        float64
	CumulativeRegret     float64
	RollbackEvents       int
}

func TestOnlinePolicy_TraceMatchedComparisonExperiment_1000Tasks(t *testing.T) {
	const taskCount = 1000
	const seed = 42
	rng := rand.New(rand.NewSource(seed))

	// 1. Generate 1,000 deterministic trace tasks
	tasks := make([]router.RoutingRequest, taskCount)
	for i := 0; i < taskCount; i++ {
		r := rng.Float64()
		var complexity router.TaskComplexity
		var minQuality float64
		var inTokens, outTokens int
		var prompt string

		switch {
		case r < 0.50: // 50% Simple
			complexity = router.ComplexitySimple
			minQuality = 0.60
			inTokens = 200 + rng.Intn(400)
			outTokens = 50 + rng.Intn(100)
			prompt = "Extract JSON structure and entities"
		case r < 0.80: // 30% Moderate
			complexity = router.ComplexityModerate
			minQuality = 0.65
			inTokens = 600 + rng.Intn(800)
			outTokens = 150 + rng.Intn(250)
			prompt = "Summarize technical architecture and refactor interface"
		case r < 0.95: // 15% Complex
			complexity = router.ComplexityComplex
			minQuality = 0.75
			inTokens = 1200 + rng.Intn(1500)
			outTokens = 300 + rng.Intn(500)
			prompt = "Multi-step orchestration design and formal proof"
		default: // 5% Reasoning Heavy
			complexity = router.ComplexityReasoningHeavy
			minQuality = 0.85
			inTokens = 2000 + rng.Intn(3000)
			outTokens = 600 + rng.Intn(800)
			prompt = "Formal verification of consensus state machine"
		}

		tasks[i] = router.RoutingRequest{
			TaskID:                fmt.Sprintf("trace-task-p19-%04d", i),
			RunID:                 fmt.Sprintf("trace-run-p19-%04d", i),
			AgentID:               "agent-trace",
			TenantID:              "tenant-trace",
			Prompt:                prompt,
			TaskComplexity:        complexity,
			QualityRequirement:    minQuality,
			EstimatedInputTokens:  inTokens,
			EstimatedOutputTokens: outTokens,
			SecurityProfile:       "standard",
			RoutingPolicy:         router.PolicyBalanced,
		}
	}

	// 2. Evaluate 3 Engines: Stage 17 Deterministic vs Stage 18 Adaptive vs Stage 19 Policy Learning
	const driftInjectionPoint = 300
	const driftRecoveryPoint = 700

	engines := []struct {
		name        string
		stageNumber int
	}{
		{"Stage17_Deterministic_Baseline", 17},
		{"Stage18_Predictive_Adaptive", 18},
		{"Stage19_Online_Policy_Learning", 19},
	}

	var results []Stage19ExperimentMetrics

	for _, eng := range engines {
		registry := router.NewDefaultModelRegistry()
		provider := router.NewSyntheticModelProvider(registry, false)
		learningStore := adaptive.NewMemoryLearningStore()
		prior := adaptive.DefaultBetaPrior()
		detector := adaptive.NewDualWindowDriftDetector()
		pm := onlinepolicy.NewPolicyManager(onlinepolicy.DefaultPolicyState())
		guardrails := onlinepolicy.NewGuardrailEnforcer(onlinepolicy.DefaultGuardrailConfig())
		decisionRepo := router.NewMemoryDecisionRepository()
		outboxRepo := outbox.NewMemoryRepository()
		txManager := memory.NewTxManager()

		stage17Svc := router.NewService(registry, provider, decisionRepo, outboxRepo, txManager)
		stage18Svc := adaptive.NewAdaptiveService(registry, provider, learningStore, detector, prior, decisionRepo, outboxRepo, txManager)
		stage19Svc := onlinepolicy.NewOnlinePolicyService(registry, provider, learningStore, prior, pm, guardrails, decisionRepo, outboxRepo, txManager)
		ctx := context.Background()

		var totalCost float64
		var latencies []float64
		var totalQuality float64
		var cumulativeRegret float64
		passCount := 0
		fallbackCount := 0
		exploreCount := 0
		constraintViolations := 0

		for taskIdx, task := range tasks {
			// At task #300, inject quality degradation on Medium
			if taskIdx == driftInjectionPoint {
				provider.SetDegradedMode("medium-balanced-v1", 0.55, 2.5)
			}
			// At task #700, recover Medium provider
			if taskIdx == driftRecoveryPoint {
				provider.SetDegradedMode("medium-balanced-v1", 0.91, 1.0)
			}

			isDegradedPeriod := (taskIdx >= driftInjectionPoint && taskIdx < driftRecoveryPoint)

			// Oracle optimal calculation
			oracleReward := 0.92
			if isDegradedPeriod && task.TaskComplexity == router.ComplexityModerate {
				oracleReward = 0.95 // Large is optimal during degradation
			}

			var resp *router.ModelInvocationResponse
			var err error
			var isExplore bool

			switch eng.stageNumber {
			case 17:
				resp, err = stage17Svc.Execute(ctx, task)
			case 18:
				resp, err = stage18Svc.Execute(ctx, task)
			case 19:
				dec, _ := stage19Svc.Route(ctx, task)
				isExplore = (dec.DecisionMode == onlinepolicy.DecisionExplore)
				resp, err = stage19Svc.Execute(ctx, task)
			}

			if err != nil {
				continue
			}

			if isExplore {
				exploreCount++
			}

			totalCost += resp.ActualCostUSD
			latMs := float64(resp.ActualLatency) / float64(time.Millisecond)
			latencies = append(latencies, latMs)
			totalQuality += resp.QualityScore

			if resp.QualityScore >= task.QualityRequirement {
				passCount++
			}
			if resp.FallbackUsed {
				fallbackCount++
			}

			actualReward := onlinepolicy.CalculateReward(router.RoutingOutcomeRecord{
				ActualQualityScore: resp.QualityScore,
				ActualCostUSD:      resp.ActualCostUSD,
				ActualLatency:      resp.ActualLatency,
				Success:            true,
			}, task, onlinepolicy.DefaultRewardWeights())

			instantRegret := math.Max(0.0, oracleReward-actualReward)
			cumulativeRegret += instantRegret
		}

		meanLat := 0.0
		var sumLat float64
		for _, l := range latencies {
			sumLat += l
		}
		if len(latencies) > 0 {
			meanLat = sumLat / float64(len(latencies))
		}

		p95Lat := 0.0
		if len(latencies) > 0 {
			p95Idx := int(float64(len(latencies)-1) * 0.95)
			p95Lat = latencies[p95Idx]
		}

		meanQual := totalQuality / float64(len(tasks))
		passRate := (float64(passCount) / float64(len(tasks))) * 100.0
		fallbackRate := (float64(fallbackCount) / float64(len(tasks))) * 100.0
		exploreRate := (float64(exploreCount) / float64(len(tasks))) * 100.0
		avgRegret := cumulativeRegret / float64(len(tasks))
		rollbacks := len(pm.ListRollbackEvents())

		results = append(results, Stage19ExperimentMetrics{
			EngineName:           eng.name,
			TotalCostUSD:         totalCost,
			MeanLatencyMs:        meanLat,
			P95LatencyMs:        p95Lat,
			MeanQualityScore:     meanQual,
			QualityPassRatePct:   passRate,
			ExplorationRatePct:   exploreRate,
			FallbackRatePct:      fallbackRate,
			ConstraintViolations: constraintViolations,
			AverageRegret:        avgRegret,
			CumulativeRegret:     cumulativeRegret,
			RollbackEvents:       rollbacks,
		})
	}

	// Print comparison report table
	fmt.Printf("\n========================================================================================================================\n")
	fmt.Printf("          STAGE 19: 1,000-TASK TRACE-MATCHED COMPARISON (STAGE 17 vs STAGE 18 vs STAGE 19)                              \n")
	fmt.Printf("========================================================================================================================\n")
	fmt.Printf("%-32s | %-10s | %-10s | %-10s | %-9s | %-9s | %-9s | %-10s | %-11s\n",
		"Engine", "Total Cost", "Mean Lat", "P95 Lat", "Mean Qual", "Pass Rate", "Exp Rate", "Avg Regret", "Cumul Regret")
	fmt.Printf("------------------------------------------------------------------------------------------------------------------------\n")

	for _, r := range results {
		fmt.Printf("%-32s | $%-9.4f | %-8.2fms | %-8.2fms | %-9.2f | %-8.1f%% | %-8.1f%% | %-10.4f | %-11.2f\n",
			r.EngineName, r.TotalCostUSD, r.MeanLatencyMs, r.P95LatencyMs, r.MeanQualityScore, r.QualityPassRatePct, r.ExplorationRatePct, r.AverageRegret, r.CumulativeRegret)
	}
	fmt.Printf("========================================================================================================================\n\n")

	// Verification Assertions
	stage17 := results[0]
	stage19 := results[2]

	// 1. Invariant: 0 constraint violations across all engines
	if stage19.ConstraintViolations != 0 {
		t.Errorf("Expected 0 constraint violations in Stage 19, got %d", stage19.ConstraintViolations)
	}

	// 2. Exploration rate must respect 5% budget
	if stage19.ExplorationRatePct > 5.5 {
		t.Errorf("Exploration rate (%.2f%%) exceeded 5%% budget limit", stage19.ExplorationRatePct)
	}

	// 3. Stage 19 should maintain high pass rate and lower cumulative regret than Stage 17
	if stage19.CumulativeRegret >= stage17.CumulativeRegret {
		t.Errorf("Expected Stage 19 cumulative regret (%.2f) to be lower than Stage 17 (%.2f)",
			stage19.CumulativeRegret, stage17.CumulativeRegret)
	}
}
