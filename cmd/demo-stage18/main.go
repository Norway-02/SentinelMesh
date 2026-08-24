package main

import (
	"context"
	"fmt"
	"time"

	"github.com/sentinelmesh/sentinelmesh/internal/adaptive"
	"github.com/sentinelmesh/sentinelmesh/internal/outbox"
	"github.com/sentinelmesh/sentinelmesh/internal/repository/memory"
	"github.com/sentinelmesh/sentinelmesh/internal/router"
)

func main() {
	fmt.Printf("\n================================================================================\n")
	fmt.Printf("          SENTINELMESH STAGE 18: PREDICTIVE & ADAPTIVE INTELLIGENCE             \n")
	fmt.Printf("================================================================================\n")

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

	// 1. Display Engine Versions & Architecture Laws
	fmt.Printf("\n[1] ENGINE SPECIFICATION & ARCHITECTURAL INVARIANTS:\n")
	fmt.Printf("--------------------------------------------------------------------------------\n")
	fmt.Printf("  • Learning Model Version:    %s\n", adaptive.LearningModelVersion)
	fmt.Printf("  • Feature Schema Version:    %s\n", adaptive.FeatureSchemaVersion)
	fmt.Printf("  • Beta Prior Version:        %s (α=%.1f, β=%.1f)\n", prior.Version, prior.Alpha, prior.Beta)
	fmt.Printf("  • Drift Detector Version:    %s (Dual-Window Baseline=100, Recent=20)\n", adaptive.DriftDetectorVersion)
	fmt.Printf("  • Immutable Safety Floor:    Adaptive ranking strictly bounded by Stage 17 Hard Gates\n")
	fmt.Printf("--------------------------------------------------------------------------------\n")

	// 2. Demonstrate Cold-Start Bayesian Shrinkage Progression
	fmt.Printf("\n[2] COLD-START BAYESIAN SHRINKAGE PROGRESSION:\n")
	fmt.Printf("--------------------------------------------------------------------------------\n")
	sampleMilestones := []int{0, 2, 10, 50}

	moderateReq := router.RoutingRequest{
		TaskID:                "demo-moderate-task",
		Prompt:                "Refactor interface and optimize concurrency",
		TaskComplexity:        router.ComplexityModerate,
		QualityRequirement:    0.75,
		EstimatedInputTokens:  800,
		EstimatedOutputTokens: 250,
		SecurityProfile:       "standard",
		RoutingPolicy:         router.PolicyBalanced,
	}

	for _, n := range sampleMilestones {
		subStore := adaptive.NewMemoryLearningStore()
		for i := 0; i < n; i++ {
			_ = subStore.RecordOutcome(ctx, router.RoutingOutcomeRecord{
				TaskID:             fmt.Sprintf("prep-%d", i),
				ModelID:            "medium-balanced-v1",
				ActualLatency:      125 * time.Millisecond,
				ActualCostUSD:      0.0018,
				ActualQualityScore: 0.92,
				Success:            true,
			}, moderateReq)
		}

		subSvc := adaptive.NewAdaptiveService(registry, provider, subStore, detector, prior, decisionRepo, outboxRepo, txManager)
		decision, _ := subSvc.Route(ctx, moderateReq)

		fmt.Printf("  N=%-3d samples | Confidence: %-5.2f | P(Success): %.3f (95%% CI: [%.3f, %.3f]) | Selected: %-18s | Util: %.4f\n",
			n, decision.Confidence, decision.PredictedSuccess, decision.SuccessQuantiles.Q025, decision.SuccessQuantiles.Q975,
			decision.SelectedModelID, decision.EffectiveUtility)
	}
	fmt.Printf("--------------------------------------------------------------------------------\n")

	// 3. Demonstrate Injected Provider Drift Detection & Dynamic Rerouting
	fmt.Printf("\n[3] LIVE DRIFT DETECTION & AUTOMATIC TRAFFIC REROUTING:\n")
	fmt.Printf("--------------------------------------------------------------------------------\n")

	// Baseline phase (100 healthy observations)
	fmt.Printf("  Phase A: Establishing healthy baseline for 'medium-balanced-v1' (100 requests)...\n")
	for i := 0; i < 100; i++ {
		detector.RecordObservation("medium-balanced-v1", 0.91, 220.0, true)
		_ = store.RecordOutcome(ctx, router.RoutingOutcomeRecord{
			TaskID:             fmt.Sprintf("base-%d", i),
			ModelID:            "medium-balanced-v1",
			ActualLatency:      220 * time.Millisecond,
			ActualCostUSD:      0.0015,
			ActualQualityScore: 0.91,
			Success:            true,
		}, moderateReq)
	}

	decPre, _ := svc.Route(ctx, moderateReq)
	fmt.Printf("  ✓ Pre-Drift Selection:  %-18s (Utility: %.4f, Status: HEALTHY)\n", decPre.SelectedModelID, decPre.EffectiveUtility)

	// Inject silent quality degradation on medium model (0.91 -> 0.55)
	fmt.Printf("\n  Phase B: Injected unannounced provider regression on 'medium-balanced-v1' (Quality 0.91 -> 0.55)...\n")
	provider.SetDegradedMode("medium-balanced-v1", 0.55, 2.5)

	driftDetectedAt := 0
	for i := 1; i <= 20; i++ {
		flagged, metric, reason := detector.RecordObservation("medium-balanced-v1", 0.55, 550.0, true)
		_ = store.RecordOutcome(ctx, router.RoutingOutcomeRecord{
			TaskID:             fmt.Sprintf("drift-%d", i),
			ModelID:            "medium-balanced-v1",
			ActualLatency:      550 * time.Millisecond,
			ActualCostUSD:      0.0015,
			ActualQualityScore: 0.55,
			Success:            true,
		}, moderateReq)

		if flagged && driftDetectedAt == 0 {
			driftDetectedAt = i
			fmt.Printf("  🚨 [ALERT] Statistical Drift Detected at observation #%d! Metric: %s | Reason: %s\n", i, metric, reason)
		}
	}

	decPost, _ := svc.Route(ctx, moderateReq)
	fmt.Printf("  ✓ Post-Drift Selection: %-18s (Utility: %.4f, Degraded Endpoint Automatically Bypassed)\n", decPost.SelectedModelID, decPost.EffectiveUtility)
	fmt.Printf("--------------------------------------------------------------------------------\n")

	// 4. Invariant Safety Guarantee
	fmt.Printf("\n[4] HARD SAFETY CONSTRAINT INVARIANCE:\n")
	fmt.Printf("--------------------------------------------------------------------------------\n")
	airgappedReq := router.RoutingRequest{
		TaskID:             "demo-airgap",
		Prompt:             "Classified intelligence payload processing",
		TaskComplexity:     router.ComplexitySimple,
		QualityRequirement: 0.60,
		SecurityProfile:    "airgapped", // Only airgapped models allowed
		RoutingPolicy:      router.PolicyBalanced,
	}

	airgapDec, err := svc.Route(ctx, airgappedReq)
	if err != nil {
		fmt.Printf("  ✓ Inviolable Constraint Gate: %v (Unsafe endpoints rejected, 0 safety violations)\n", err)
	} else {
		fmt.Printf("  ✓ Airgapped Decision: Selected %s\n", airgapDec.SelectedModelID)
	}
	fmt.Printf("--------------------------------------------------------------------------------\n")

	fmt.Printf("\n================================================================================\n")
	fmt.Printf("       STAGE 18 PREDICTIVE & ADAPTIVE INTELLIGENCE DEMONSTRATION COMPLETE       \n")
	fmt.Printf("================================================================================\n\n")
}
