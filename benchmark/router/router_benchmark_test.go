package router_test

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/sentinelmesh/sentinelmesh/internal/outbox"
	"github.com/sentinelmesh/sentinelmesh/internal/repository/memory"
	"github.com/sentinelmesh/sentinelmesh/internal/router"
)

func BenchmarkRouter_DecisionLatency(b *testing.B) {
	registry := router.NewDefaultModelRegistry()
	provider := router.NewSyntheticModelProvider(registry, false)
	decisionRepo := router.NewMemoryDecisionRepository()
	outboxRepo := outbox.NewMemoryRepository()
	txManager := memory.NewTxManager()
	svc := router.NewService(registry, provider, decisionRepo, outboxRepo, txManager)
	ctx := context.Background()

	scenarios := []struct {
		name       string
		complexity router.TaskComplexity
		policy     router.RoutingPolicy
	}{
		{"Simple_CostOptimized", router.ComplexitySimple, router.PolicyCostOptimized},
		{"Moderate_Balanced", router.ComplexityModerate, router.PolicyBalanced},
		{"Complex_QualityOptimized", router.ComplexityComplex, router.PolicyQualityOptimized},
		{"Reasoning_LatencyOptimized", router.ComplexityReasoningHeavy, router.PolicyLatencyOptimized},
	}

	for _, s := range scenarios {
		b.Run(s.name, func(b *testing.B) {
			req := router.RoutingRequest{
				TaskID:                "bench-task",
				Prompt:                "sample benchmark prompt text with moderate length",
				TaskComplexity:        s.complexity,
				QualityRequirement:    0.70,
				EstimatedInputTokens:  500,
				EstimatedOutputTokens: 250,
				SecurityProfile:       "standard",
				RoutingPolicy:         s.policy,
			}

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, err := svc.Route(ctx, req)
				if err != nil {
					b.Fatalf("Route failed: %v", err)
				}
			}
		})
	}
}

func BenchmarkRouter_EndToEndInvocation(b *testing.B) {
	registry := router.NewDefaultModelRegistry()
	provider := router.NewSyntheticModelProvider(registry, false)
	decisionRepo := router.NewMemoryDecisionRepository()
	outboxRepo := outbox.NewMemoryRepository()
	txManager := memory.NewTxManager()
	svc := router.NewService(registry, provider, decisionRepo, outboxRepo, txManager)
	ctx := context.Background()

	req := router.RoutingRequest{
		TaskID:                "bench-e2e-task",
		RunID:                 "run-bench",
		AgentID:               "agent-1",
		TenantID:              "tenant-1",
		Prompt:                "sample prompt for synthetic model execution",
		TaskComplexity:        router.ComplexityModerate,
		QualityRequirement:    0.75,
		EstimatedInputTokens:  600,
		EstimatedOutputTokens: 200,
		SecurityProfile:       "standard",
		RoutingPolicy:         router.PolicyBalanced,
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := svc.Execute(ctx, req)
		if err != nil {
			b.Fatalf("Execute failed: %v", err)
		}
	}
}

type PolicyComparisonMetrics struct {
	Policy             string
	TotalCostUSD       float64
	MeanLatencyMs      float64
	P95LatencyMs       float64
	MeanQualityScore   float64
	QualityPassRatePct float64
	FallbackRatePct    float64
	CostSavingsPct     float64
	LatencyReductionPct float64
}

func TestRouter_PolicyComparisonExperiment_1000Tasks(t *testing.T) {
	const taskCount = 1000
	const seed = 42
	rng := rand.New(rand.NewSource(seed))

	// Generate 1,000 heterogeneous tasks
	tasks := make([]router.RoutingRequest, taskCount)
	for i := 0; i < taskCount; i++ {
		r := rng.Float64()
		var complexity router.TaskComplexity
		var minQuality float64
		var inTokens, outTokens int

		switch {
		case r < 0.50: // 50% Simple
			complexity = router.ComplexitySimple
			minQuality = 0.60
			inTokens = 200 + rng.Intn(400)
			outTokens = 50 + rng.Intn(100)
		case r < 0.80: // 30% Moderate
			complexity = router.ComplexityModerate
			minQuality = 0.65
			inTokens = 600 + rng.Intn(800)
			outTokens = 150 + rng.Intn(250)
		case r < 0.95: // 15% Complex
			complexity = router.ComplexityComplex
			minQuality = 0.75
			inTokens = 1200 + rng.Intn(1500)
			outTokens = 300 + rng.Intn(500)
		default: // 5% Reasoning Heavy
			complexity = router.ComplexityReasoningHeavy
			minQuality = 0.85
			inTokens = 2000 + rng.Intn(3000)
			outTokens = 600 + rng.Intn(800)
		}

		tasks[i] = router.RoutingRequest{
			TaskID:                fmt.Sprintf("task-%04d", i),
			RunID:                 fmt.Sprintf("run-%04d", i),
			AgentID:               "agent-bench",
			TenantID:              "tenant-bench",
			Prompt:                fmt.Sprintf("Prompt payload for synthetic task %d", i),
			TaskComplexity:        complexity,
			QualityRequirement:    minQuality,
			EstimatedInputTokens:  inTokens,
			EstimatedOutputTokens: outTokens,
			SecurityProfile:       "standard",
		}
	}

	policies := []struct {
		name       string
		policy     router.RoutingPolicy
		pinnedID   string
	}{
		{"Static_Large_Baseline", router.PolicyStatic, "large-reasoning-v1"},
		{"Static_Small_Baseline", router.PolicyStatic, "small-fast-v1"},
		{"Cost_Optimized", router.PolicyCostOptimized, ""},
		{"Latency_Optimized", router.PolicyLatencyOptimized, ""},
		{"Quality_Optimized", router.PolicyQualityOptimized, ""},
		{"Balanced_Pareto_Router", router.PolicyBalanced, ""},
	}

	var results []PolicyComparisonMetrics
	var baselineCost, baselineP95Lat float64

	for pIdx, p := range policies {
		registry := router.NewDefaultModelRegistry()
		provider := router.NewSyntheticModelProvider(registry, false)
		decisionRepo := router.NewMemoryDecisionRepository()
		outboxRepo := outbox.NewMemoryRepository()
		txManager := memory.NewTxManager()
		svc := router.NewService(registry, provider, decisionRepo, outboxRepo, txManager)
		ctx := context.Background()

		var totalCost float64
		var latencies []float64
		var totalQuality float64
		passCount := 0
		fallbackCount := 0

		for _, task := range tasks {
			taskReq := task
			taskReq.RoutingPolicy = p.policy
			taskReq.PinnedModelID = p.pinnedID

			resp, err := svc.Execute(ctx, taskReq)
			if err != nil {
				continue
			}

			totalCost += resp.ActualCostUSD
			latMs := float64(resp.ActualLatency) / float64(time.Millisecond)
			latencies = append(latencies, latMs)
			totalQuality += resp.QualityScore

			if resp.QualityScore >= taskReq.QualityRequirement {
				passCount++
			}
			if resp.FallbackUsed {
				fallbackCount++
			}
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
			idx := int(float64(len(latencies)-1) * 0.95)
			p95Lat = latencies[idx]
		}

		meanQual := 0.0
		if len(tasks) > 0 {
			meanQual = totalQuality / float64(len(tasks))
		}

		passRate := (float64(passCount) / float64(len(tasks))) * 100.0
		fallbackRate := (float64(fallbackCount) / float64(len(tasks))) * 100.0

		if pIdx == 0 {
			baselineCost = totalCost
			baselineP95Lat = p95Lat
		}

		costSavings := 0.0
		if baselineCost > 0 {
			costSavings = ((baselineCost - totalCost) / baselineCost) * 100.0
		}

		latReduction := 0.0
		if baselineP95Lat > 0 {
			latReduction = ((baselineP95Lat - p95Lat) / baselineP95Lat) * 100.0
		}

		metrics := PolicyComparisonMetrics{
			Policy:              p.name,
			TotalCostUSD:        totalCost,
			MeanLatencyMs:       meanLat,
			P95LatencyMs:        p95Lat,
			MeanQualityScore:    meanQual,
			QualityPassRatePct:  passRate,
			FallbackRatePct:     fallbackRate,
			CostSavingsPct:      costSavings,
			LatencyReductionPct: latReduction,
		}
		results = append(results, metrics)
	}

	// Print comparison table
	fmt.Printf("\n========================================================================================================================\n")
	fmt.Printf("                    STAGE 17: 1,000 HETEROGENEOUS TASKS POLICY COMPARISON EXPERIMENT                                     \n")
	fmt.Printf("========================================================================================================================\n")
	fmt.Printf("%-24s | %-12s | %-12s | %-12s | %-10s | %-10s | %-12s | %-14s\n",
		"Policy", "Total Cost", "Mean Latency", "P95 Latency", "Mean Qual", "Pass Rate", "Cost Savings", "Lat Reduction")
	fmt.Printf("------------------------------------------------------------------------------------------------------------------------\n")

	for _, r := range results {
		fmt.Printf("%-24s | $%-11.4f | %-10.2fms | %-10.2fms | %-10.2f | %-9.1f%% | %-11.1f%% | %-13.1f%%\n",
			r.Policy, r.TotalCostUSD, r.MeanLatencyMs, r.P95LatencyMs, r.MeanQualityScore, r.QualityPassRatePct, r.CostSavingsPct, r.LatencyReductionPct)
	}
	fmt.Printf("========================================================================================================================\n\n")

	// Verification assertions
	balanced := results[5]
	if balanced.TotalCostUSD >= baselineCost {
		t.Errorf("Expected Balanced Router to be cheaper than Static Large baseline")
	}
	if balanced.QualityPassRatePct < 90.0 {
		t.Errorf("Expected Balanced Router quality pass rate >= 90%%, got %.1f%%", balanced.QualityPassRatePct)
	}
}
