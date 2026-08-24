package main

import (
	"context"
	"fmt"
	"time"

	"github.com/sentinelmesh/sentinelmesh/internal/outbox"
	"github.com/sentinelmesh/sentinelmesh/internal/repository/memory"
	"github.com/sentinelmesh/sentinelmesh/internal/router"
)

func main() {
	fmt.Printf("\n================================================================================\n")
	fmt.Printf("          SENTINELMESH STAGE 17: MODEL ROUTER & ADAPTIVE INTELLIGENCE           \n")
	fmt.Printf("================================================================================\n")

	registry := router.NewDefaultModelRegistry()
	provider := router.NewSyntheticModelProvider(registry, false)
	decisionRepo := router.NewMemoryDecisionRepository()
	outboxRepo := outbox.NewMemoryRepository()
	txManager := memory.NewTxManager()
	svc := router.NewService(registry, provider, decisionRepo, outboxRepo, txManager)
	ctx := context.Background()

	// 1. Display Registered Models
	models, _ := registry.ListModels(ctx)
	fmt.Printf("\n[1] REGISTERED MODEL CATALOG (%s | %s):\n", registry.Version(), router.PolicyVersion)
	fmt.Printf("--------------------------------------------------------------------------------\n")
	fmt.Printf("%-18s | %-8s | %-12s | %-12s | %-10s | %-10s\n", "Model ID", "Tier", "Input $/1k", "Output $/1k", "P50 Latency", "Context")
	fmt.Printf("--------------------------------------------------------------------------------\n")
	for _, m := range models {
		fmt.Printf("%-18s | %-8s | $%-11.5f | $%-11.5f | %-8.1fms | %-10d\n",
			m.ID, m.Tier, m.CostPer1kInputTokens, m.CostPer1kOutputTokens, m.NominalP50LatencyMs, m.ContextWindow)
	}
	fmt.Printf("--------------------------------------------------------------------------------\n")

	// 2. Demonstrate Deterministic Routing across 4 Task Classes with Explainability
	tasks := []struct {
		name       string
		complexity router.TaskComplexity
		policy     router.RoutingPolicy
		prompt     string
		inTokens   int
		outTokens  int
		minQual    float64
	}{
		{"Entity Extraction", router.ComplexitySimple, router.PolicyCostOptimized, "Extract entity JSON from string", 350, 80, 0.60},
		{"Code Refactoring", router.ComplexityModerate, router.PolicyBalanced, "Refactor Go handler to use interfaces", 900, 300, 0.85},
		{"Architecture Design", router.ComplexityComplex, router.PolicyBalanced, "Design distributed consensus fencing token flow", 1800, 600, 0.90},
		{"Formal Proof", router.ComplexityReasoningHeavy, router.PolicyQualityOptimized, "Formal verification of raft state transitions", 3000, 1000, 0.92},
	}

	fmt.Printf("\n[2] REAL-TIME DETERMINISTIC ROUTING & EXPLAINABILITY AUDIT:\n")
	for idx, t := range tasks {
		req := router.RoutingRequest{
			TaskID:                fmt.Sprintf("demo-task-%d", idx+1),
			RunID:                 fmt.Sprintf("demo-run-%d", idx+1),
			AgentID:               "agent-demo",
			TenantID:              "tenant-demo",
			Prompt:                t.prompt,
			TaskComplexity:        t.complexity,
			QualityRequirement:    t.minQual,
			EstimatedInputTokens:  t.inTokens,
			EstimatedOutputTokens: t.outTokens,
			SecurityProfile:       "standard",
			RoutingPolicy:         t.policy,
		}

		resp, err := svc.Execute(ctx, req)
		if err != nil {
			fmt.Printf("  • Task %d (%s): FAILED (%v)\n", idx+1, t.name, err)
			continue
		}

		decision, _ := decisionRepo.GetDecision(ctx, req.TaskID)
		fmt.Printf("  • Task %d [%s] (Complexity: %s, Policy: %s, Schema: %s):\n",
			idx+1, t.name, t.complexity, t.policy, decision.AlgorithmVersion)
		fmt.Printf("      → Selected Model: %s (Tier: %s | Score: %.3f | Pareto: %v)\n",
			decision.SelectedModelID, decision.SelectedTier, decision.FinalScore, decision.IsParetoOptimal)
		fmt.Printf("      → Score Breakdown: Qual=%.2f, Cost=%.2f, Lat=%.2f, Rel=%.2f\n",
			decision.ScoreBreakdown.Quality, decision.ScoreBreakdown.Cost, decision.ScoreBreakdown.Latency, decision.ScoreBreakdown.Reliability)
		if len(decision.Rejections) > 0 {
			fmt.Printf("      → Rejections Auditing:\n")
			for _, r := range decision.Rejections {
				fmt.Printf("          - %s: %s (%s)\n", r.ModelID, r.Reason, r.Details)
			}
		}
		fmt.Printf("      → Invocation Telemetry: Cost=$%.5f | Latency=%.1fms | Quality=%.2f\n",
			resp.ActualCostUSD, float64(resp.ActualLatency)/float64(time.Millisecond), resp.QualityScore)
		fmt.Printf("      → Fallback Chain: %v\n\n", decision.FallbackCandidates)
	}

	// 3. Policy Separation on Identical Workload
	fmt.Printf("[3] POLICY SEPARATION ON IDENTICAL MODERATE WORKLOAD:\n")
	policies := []router.RoutingPolicy{
		router.PolicyCostOptimized,
		router.PolicyLatencyOptimized,
		router.PolicyQualityOptimized,
		router.PolicyBalanced,
	}

	for _, p := range policies {
		req := router.RoutingRequest{
			TaskID:                fmt.Sprintf("demo-policy-%s", p),
			RunID:                 "demo-policy-run",
			Prompt:                "summarize technical RFC",
			TaskComplexity:        router.ComplexityModerate,
			QualityRequirement:    0.70,
			EstimatedInputTokens:  1000,
			EstimatedOutputTokens: 500,
			SecurityProfile:       "standard",
			RoutingPolicy:         p,
		}
		d, err := svc.Route(ctx, req)
		if err != nil {
			fmt.Printf("  • Policy %-18s: FAILED (%v)\n", p, err)
			continue
		}
		fmt.Printf("  • Policy %-20s → Selected: %-18s (Est Cost: $%.5f, Est Lat: %.1fms, Score: %.3f)\n",
			p, d.SelectedModelID, d.EstimatedCostUSD, d.EstimatedLatencyMs, d.FinalScore)
	}

	// 4. Demonstrate Cascading Multi-Step Fallback
	fmt.Printf("\n[4] CASCADING FAULT INJECTION & AUTOMATIC MULTI-STEP FAILOVER:\n")
	fmt.Printf("  • Injecting HTTP 429 on Primary (large) and HTTP 503 on Secondary (medium)...\n")
	provider.SetFault("large-reasoning-v1", router.FaultRateLimit, 3)
	provider.SetFault("medium-balanced-v1", router.FaultServerError, 3)

	failoverReq := router.RoutingRequest{
		TaskID:                "demo-cascade-task",
		RunID:                 "demo-cascade-run",
		AgentID:               "agent-demo",
		TenantID:              "tenant-demo",
		Prompt:                "Execute consensus round with fallback",
		TaskComplexity:        router.ComplexityReasoningHeavy,
		QualityRequirement:    0.10, // Allows small as emergency fallback
		EstimatedInputTokens:  1000,
		EstimatedOutputTokens: 500,
		SecurityProfile:       "standard",
		RoutingPolicy:         router.PolicyQualityOptimized,
	}

	failoverResp, err := svc.Execute(ctx, failoverReq)
	if err != nil {
		fmt.Printf("  • Cascade execution failed: %v\n", err)
	} else {
		fmt.Printf("  ✓ Cascading Failover Succeeded!\n")
		fmt.Printf("      → Primary 'large-reasoning-v1' failed (429 Rate Limit) → tripped Breaker\n")
		fmt.Printf("      → Secondary 'medium-balanced-v1' failed (503 Server Error) → tripped Breaker\n")
		fmt.Printf("      → Successfully Executed on Tertiary: %s (FallbackUsed: %v, Attempt: %d)\n",
			failoverResp.ModelID, failoverResp.FallbackUsed, failoverResp.AttemptNumber)
		fmt.Printf("      → Actual Cost: $%.5f | Latency: %.1fms | Quality: %.2f\n",
			failoverResp.ActualCostUSD, float64(failoverResp.ActualLatency)/float64(time.Millisecond), failoverResp.QualityScore)
	}

	fmt.Println("\n================================================================================")
	fmt.Println("                  STAGE 17 DEMO COMPLETED SUCCESSFULLY                          ")
	fmt.Println("================================================================================")
}
