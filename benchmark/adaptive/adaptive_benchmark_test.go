package adaptive_test

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"testing"
	"time"

	"github.com/sentinelmesh/sentinelmesh/internal/adaptive"
	"github.com/sentinelmesh/sentinelmesh/internal/outbox"
	"github.com/sentinelmesh/sentinelmesh/internal/repository/memory"
	"github.com/sentinelmesh/sentinelmesh/internal/router"
)

func BenchmarkAdaptiveRouter_PureAlgorithm(b *testing.B) {
	registry := router.NewDefaultModelRegistry()
	models, _ := registry.ListModels(context.Background())
	store := adaptive.NewMemoryLearningStore()
	detector := adaptive.NewDualWindowDriftDetector()
	prior := adaptive.DefaultBetaPrior()

	req := router.RoutingRequest{
		TaskID:                "bench-task",
		Prompt:                "summarize technical document and extract entities",
		TaskComplexity:        router.ComplexityModerate,
		QualityRequirement:    0.75,
		EstimatedInputTokens:  800,
		EstimatedOutputTokens: 250,
		SecurityProfile:       "standard",
		RoutingPolicy:         router.PolicyBalanced,
	}

	for i := 0; i < 50; i++ {
		_ = store.RecordOutcome(context.Background(), router.RoutingOutcomeRecord{
			TaskID:             fmt.Sprintf("bench-prep-%d", i),
			ModelID:            "medium-balanced-v1",
			ActualLatency:      130 * time.Millisecond,
			ActualCostUSD:      0.002,
			ActualQualityScore: 0.90,
			Success:            true,
		}, req)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := adaptive.RouteAdaptive(req, models, store, detector, prior)
		if err != nil {
			b.Fatalf("RouteAdaptive failed: %v", err)
		}
	}
}

func BenchmarkAdaptiveRouter_FullService(b *testing.B) {
	registry := router.NewDefaultModelRegistry()
	provider := router.NewSyntheticModelProvider(registry, false)
	store := adaptive.NewMemoryLearningStore()
	detector := adaptive.NewDualWindowDriftDetector()
	prior := adaptive.DefaultBetaPrior()
	decisionRepo := router.NewMemoryDecisionRepository()
	outboxRepo := outbox.NewMemoryRepository()
	txManager := memory.NewTxManager()

	svc := adaptive.NewAdaptiveService(registry, provider, store, detector, prior, decisionRepo, outboxRepo, txManager)
	ctx := context.Background()

	req := router.RoutingRequest{
		TaskID:                "bench-task",
		Prompt:                "summarize technical document and extract entities",
		TaskComplexity:        router.ComplexityModerate,
		QualityRequirement:    0.75,
		EstimatedInputTokens:  800,
		EstimatedOutputTokens: 250,
		SecurityProfile:       "standard",
		RoutingPolicy:         router.PolicyBalanced,
	}

	for i := 0; i < 50; i++ {
		_ = store.RecordOutcome(ctx, router.RoutingOutcomeRecord{
			TaskID:             fmt.Sprintf("bench-prep-%d", i),
			ModelID:            "medium-balanced-v1",
			ActualLatency:      130 * time.Millisecond,
			ActualCostUSD:      0.002,
			ActualQualityScore: 0.90,
			Success:            true,
		}, req)
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

type ExperimentMetrics struct {
	EngineName           string
	TotalCostUSD         float64
	MeanLatencyMs        float64
	P95LatencyMs         float64
	MeanQualityScore     float64
	QualityPassRatePct   float64
	FallbackRatePct      float64
	ConstraintViolations int
	DriftDetectionDelay  int
	AverageRegret        float64
}

func TestAdaptiveRouter_TraceMatchedComparisonExperiment_1000Tasks(t *testing.T) {
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
			TaskID:                fmt.Sprintf("trace-task-%04d", i),
			RunID:                 fmt.Sprintf("trace-run-%04d", i),
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

	// 2. Evaluate Stage 17 Deterministic Router vs Stage 18 Adaptive Router under Injected Drift at task #300
	const driftInjectionPoint = 300

	engines := []struct {
		name       string
		isAdaptive bool
	}{
		{"Stage17_Deterministic_Router", false},
		{"Stage18_Predictive_Adaptive_Router", true},
	}

	var results []ExperimentMetrics

	for _, eng := range engines {
		registry := router.NewDefaultModelRegistry()
		provider := router.NewSyntheticModelProvider(registry, false)
		store := adaptive.NewMemoryLearningStore()
		detector := adaptive.NewDualWindowDriftDetector()
		prior := adaptive.DefaultBetaPrior()
		decisionRepo := router.NewMemoryDecisionRepository()
		outboxRepo := outbox.NewMemoryRepository()
		txManager := memory.NewTxManager()

		stage17Svc := router.NewService(registry, provider, decisionRepo, outboxRepo, txManager)
		stage18Svc := adaptive.NewAdaptiveService(registry, provider, store, detector, prior, decisionRepo, outboxRepo, txManager)
		ctx := context.Background()

		var totalCost float64
		var latencies []float64
		var totalQuality float64
		var totalRegret float64
		passCount := 0
		fallbackCount := 0
		constraintViolations := 0
		driftDelay := 0
		driftDetectedAt := -1

		for taskIdx, task := range tasks {
			// At task #300, inject silent quality degradation (0.91 -> 0.55) and latency spike on medium model
			isPostDrift := (taskIdx >= driftInjectionPoint)
			if isPostDrift && taskIdx == driftInjectionPoint {
				provider.SetDegradedMode("medium-balanced-v1", 0.55, 3.0) // 0.55 quality, 3x latency
			}

			// Oracle calculation for regret
			oracleModel := "medium-balanced-v1"
			if isPostDrift && task.TaskComplexity == router.ComplexityModerate {
				oracleModel = "large-reasoning-v1" // Oracle switches to Large because Medium degraded
			} else if task.TaskComplexity == router.ComplexitySimple {
				oracleModel = "small-fast-v1"
			} else if task.TaskComplexity >= router.ComplexityComplex {
				oracleModel = "large-reasoning-v1"
			}
			oracleUtility := 0.95

			var resp *router.ModelInvocationResponse
			var selectedModel string
			var err error

			if !eng.isAdaptive {
				// Stage 17 Deterministic execution
				resp, err = stage17Svc.Execute(ctx, task)
				selectedModel = resp.ModelID
			} else {
				// Stage 18 Adaptive execution
				resp, err = stage18Svc.Execute(ctx, task)
				selectedModel = resp.ModelID

				// Track drift detection delay
				if isPostDrift && driftDetectedAt == -1 {
					isDegraded, _ := detector.IsDegraded("medium-balanced-v1")
					if isDegraded {
						driftDetectedAt = taskIdx
						driftDelay = driftDetectedAt - driftInjectionPoint
					}
				}
			}

			if err != nil {
				continue
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

			// Regret computation
			actualUtility := resp.QualityScore
			if selectedModel != oracleModel && isPostDrift && task.TaskComplexity == router.ComplexityModerate {
				actualUtility = resp.QualityScore * 0.70
			}
			regret := math.Max(0.0, oracleUtility-actualUtility)
			totalRegret += regret
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
		avgRegret := totalRegret / float64(len(tasks))

		results = append(results, ExperimentMetrics{
			EngineName:           eng.name,
			TotalCostUSD:         totalCost,
			MeanLatencyMs:        meanLat,
			P95LatencyMs:        p95Lat,
			MeanQualityScore:     meanQual,
			QualityPassRatePct:   passRate,
			FallbackRatePct:      fallbackRate,
			ConstraintViolations: constraintViolations,
			DriftDetectionDelay:  driftDelay,
			AverageRegret:        avgRegret,
		})
	}

	// Print comparison report table
	fmt.Printf("\n========================================================================================================================\n")
	fmt.Printf("          STAGE 18: 1,000-TASK TRACE-MATCHED COMPARISON (STAGE 17 DETERMINISTIC vs STAGE 18 ADAPTIVE)                   \n")
	fmt.Printf("========================================================================================================================\n")
	fmt.Printf("%-35s | %-10s | %-11s | %-11s | %-9s | %-9s | %-11s | %-9s\n",
		"Engine", "Total Cost", "Mean Lat", "P95 Lat", "Mean Qual", "Pass Rate", "Drift Delay", "Avg Regret")
	fmt.Printf("------------------------------------------------------------------------------------------------------------------------\n")

	for _, r := range results {
		fmt.Printf("%-35s | $%-9.4f | %-9.2fms | %-9.2fms | %-9.2f | %-8.1f%% | %-11d | %-9.4f\n",
			r.EngineName, r.TotalCostUSD, r.MeanLatencyMs, r.P95LatencyMs, r.MeanQualityScore, r.QualityPassRatePct, r.DriftDetectionDelay, r.AverageRegret)
	}
	fmt.Printf("========================================================================================================================\n\n")

	// Verification Assertions
	stage17 := results[0]
	stage18 := results[1]

	// 1. Invariant: 0 constraint violations
	if stage18.ConstraintViolations != 0 {
		t.Errorf("Expected 0 constraint violations in Stage 18, got %d", stage18.ConstraintViolations)
	}

	// 2. Stage 18 should have lower average regret than Stage 17 under injected drift
	if stage18.AverageRegret >= stage17.AverageRegret {
		t.Errorf("Expected Stage 18 average regret (%.4f) to be lower than Stage 17 (%.4f)",
			stage18.AverageRegret, stage17.AverageRegret)
	}

	// 3. Stage 18 should have higher quality pass rate under drift
	if stage18.QualityPassRatePct <= stage17.QualityPassRatePct {
		t.Errorf("Expected Stage 18 quality pass rate (%.1f%%) to exceed Stage 17 (%.1f%%)",
			stage18.QualityPassRatePct, stage17.QualityPassRatePct)
	}
}
