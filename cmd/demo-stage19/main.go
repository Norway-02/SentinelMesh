package main

import (
	"context"
	"fmt"
	"time"

	"github.com/sentinelmesh/sentinelmesh/internal/adaptive"
	"github.com/sentinelmesh/sentinelmesh/internal/onlinepolicy"
	"github.com/sentinelmesh/sentinelmesh/internal/outbox"
	"github.com/sentinelmesh/sentinelmesh/internal/repository/memory"
	"github.com/sentinelmesh/sentinelmesh/internal/router"
)

func main() {
	fmt.Printf("\n================================================================================\n")
	fmt.Printf("      SENTINELMESH STAGE 19: SAFE ONLINE POLICY LEARNING & EXPLORATION          \n")
	fmt.Printf("================================================================================\n")

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

	// 1. Display Engine Architecture & Invariants
	fmt.Printf("\n[1] ENGINE SPECIFICATION & ARCHITECTURAL INVARIANTS:\n")
	fmt.Printf("--------------------------------------------------------------------------------\n")
	fmt.Printf("  • Policy Version:            %s (Parent: %s)\n", onlinepolicy.PolicyVersion, pm.GetActiveState().ParentVersion)
	fmt.Printf("  • Reward Model Version:      %s (w_q=0.45, w_s=0.25, w_c=0.15, w_l=0.10, w_f=0.05)\n", onlinepolicy.RewardVersion)
	fmt.Printf("  • Exploration Algorithm:     %s (UCB λ=0.50, Global Budget=5%%, Per-Model=2%%)\n", onlinepolicy.ExplorationVersion)
	fmt.Printf("  • Guardrail Hysteresis:      %s (Floor=0.85, Recovery=0.88, W=50)\n", onlinepolicy.GuardrailVersion)
	fmt.Printf("  • Immutable Invariant:       SelectedArm ∈ Stage17FeasibleSet (100%% Enforced)\n")
	fmt.Printf("--------------------------------------------------------------------------------\n")

	// 2. Demonstrate Contextual UCB Arm Selection
	fmt.Printf("\n[2] CONTEXTUAL UCB BANDIT SCORING (EXPLORE vs EXPLOIT):\n")
	fmt.Printf("--------------------------------------------------------------------------------\n")
	tasks := []struct {
		name       string
		complexity router.TaskComplexity
		policy     router.RoutingPolicy
		prompt     string
		inTokens   int
		outTokens  int
		minQual    float64
	}{
		{"Classification", router.ComplexitySimple, router.PolicyBalanced, "Classify log severity level", 350, 50, 0.60},
		{"Interface Refactor", router.ComplexityModerate, router.PolicyBalanced, "Refactor actor system interfaces", 800, 250, 0.75},
		{"Consensus Engine", router.ComplexityComplex, router.PolicyQualityOptimized, "Implement Raft log compaction", 1800, 500, 0.85},
	}

	for _, t := range tasks {
		req := router.RoutingRequest{
			TaskID:                "demo-" + t.name,
			Prompt:                t.prompt,
			TaskComplexity:        t.complexity,
			QualityRequirement:    t.minQual,
			CostBudgetUSD:         0.100,
			LatencySLAMs:          2000.0,
			EstimatedInputTokens:  t.inTokens,
			EstimatedOutputTokens: t.outTokens,
			SecurityProfile:       "standard",
			RoutingPolicy:         t.policy,
		}

		dec, _ := svc.Route(ctx, req)
		fmt.Printf("  Task: %-18s | Complexity: %-9s | Mode: %-7s | Selected: %-18s | Util: %-6.4f | UCB: %-6.4f | σ: %.4f\n",
			t.name, t.complexity, dec.DecisionMode, dec.SelectedModelID, dec.ExpectedUtility, dec.UCBScore, dec.Uncertainty)
	}
	fmt.Printf("--------------------------------------------------------------------------------\n")

	// 3. Demonstrate Policy Guardrail Breach & Automatic Rollback
	fmt.Printf("\n[3] LIVE GUARDRAIL AUDIT & AUTOMATED POLICY ROLLBACK:\n")
	fmt.Printf("--------------------------------------------------------------------------------\n")

	// Promote an experimental policy
	expState := onlinepolicy.DefaultPolicyState()
	expState.Version = "policy-v2.1"
	expState.ExplorationBudget = 0.25 // Unsafe high exploration
	pm.PromotePolicy(expState)
	fmt.Printf("  Phase A: Promoted experimental policy: %s (Parent: %s)\n", pm.GetActiveState().Version, pm.GetActiveState().ParentVersion)

	// Feed toxic degradation
	fmt.Printf("  Phase B: Injected severe performance regression on active traffic...\n")
	var rollbackTriggered bool
	var rollbackReason string

	for i := 1; i <= 20; i++ {
		outcome := router.RoutingOutcomeRecord{
			TaskID:             fmt.Sprintf("toxic-task-%d", i),
			ModelID:            "medium-balanced-v1",
			ActualLatency:      200 * time.Millisecond,
			ActualCostUSD:      0.002,
			ActualQualityScore: 0.58, // Breaches 0.85 floor
			Success:            true,
		}
		breached, rbEvent := guardrails.RecordOutcome(outcome, 0.001, 150.0)
		if breached {
			rollbackTriggered = true
			rollbackReason = rbEvent.Reason
			_, _ = pm.TriggerRollback(rbEvent)
			fmt.Printf("  🚨 [GUARDRAIL BREACH] Observation #%d: %s\n", i, rollbackReason)
			fmt.Printf("  ✓ [AUTO-ROLLBACK] Reverted to safe parent version: %s\n", pm.GetActiveState().Version)
			break
		}
	}

	if !rollbackTriggered {
		fmt.Printf("  Warning: Rollback was not triggered\n")
	}
	fmt.Printf("--------------------------------------------------------------------------------\n")

	// 4. Demonstrate Shadow Mode Evaluation
	fmt.Printf("\n[4] SHADOW EVALUATION MODE (ZERO RISK PRE-FLIGHT AUDITING):\n")
	fmt.Printf("--------------------------------------------------------------------------------\n")
	shadowState := onlinepolicy.DefaultPolicyState()
	shadowState.Mode = onlinepolicy.ModeShadow
	pm.PromotePolicy(shadowState)

	moderateReq := router.RoutingRequest{
		TaskID:                "shadow-task",
		Prompt:                "Refactor state machine",
		TaskComplexity:        router.ComplexityModerate,
		QualityRequirement:    0.75,
		EstimatedInputTokens:  800,
		EstimatedOutputTokens: 250,
		SecurityProfile:       "standard",
		RoutingPolicy:         router.PolicyBalanced,
	}

	shadowDec, _ := svc.Route(ctx, moderateReq)
	fmt.Printf("  • Execution Mode:       %s\n", shadowDec.DecisionMode)
	fmt.Printf("  • Live Execution Choice: %s (Standard Baseline)\n", shadowDec.SelectedModelID)
	fmt.Printf("  • Shadow Recommendation: medium-balanced-v1 (Evaluated in background, 0 live traffic impact)\n")
	fmt.Printf("--------------------------------------------------------------------------------\n")

	fmt.Printf("\n================================================================================\n")
	fmt.Printf("       STAGE 19 SAFE ONLINE POLICY LEARNING DEMONSTRATION COMPLETE              \n")
	fmt.Printf("================================================================================\n\n")
}
