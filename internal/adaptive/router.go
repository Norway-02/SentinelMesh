package adaptive

import (
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/sentinelmesh/sentinelmesh/internal/router"
)

type candidateAdaptiveEval struct {
	model            router.ModelDefinition
	features         TaskFeatures
	profile          PerformanceProfile
	lookupLevel      string
	predictedSuccess float64
	successQuantiles BetaQuantiles
	qualityEstimate  QualityEstimate
	latencyEstimate  LatencyEstimate
	costEstimate     CostEstimate
	confidence       float64
	nominalScore     float64
	adaptiveScore    float64
	effectiveUtility float64
	costScore        float64
	latencyScore     float64
}

// RouteAdaptive performs uncertainty-aware, drift-adaptive model selection within the Stage 17 safe feasible set.
func RouteAdaptive(
	req router.RoutingRequest,
	models []router.ModelDefinition,
	store LearningStore,
	detector *DualWindowDriftDetector,
	prior BetaPrior,
) (AdaptiveRoutingDecision, error) {
	if len(models) == 0 {
		return AdaptiveRoutingDecision{}, fmt.Errorf("no models registered in adaptive router")
	}

	// 1. Establish Inviolable Stage 17 Feasible Set and Baseline Nominal Decision
	nominalDecision, err := router.RouteTask(req, models)
	if err != nil {
		return AdaptiveRoutingDecision{
			TaskID:               req.TaskID,
			RunID:                req.RunID,
			Policy:               req.RoutingPolicy,
			Rejections:           nominalDecision.Rejections,
			LearningModelVersion: LearningModelVersion,
			FeatureSchemaVersion: FeatureSchemaVersion,
			PriorVersion:         prior.Version,
			DriftDetectorVersion: DriftDetectorVersion,
			DecidedAt:            time.Now().UTC(),
		}, fmt.Errorf("hard constraint failure: %w", err)
	}

	// Filter models that passed Stage 17 hard constraints
	feasibleMap := make(map[string]bool)
	feasibleMap[nominalDecision.SelectedModelID] = true
	for _, fb := range nominalDecision.FallbackCandidates {
		feasibleMap[fb] = true
	}

	var feasibleModels []router.ModelDefinition
	for _, m := range models {
		if feasibleMap[m.ID] {
			feasibleModels = append(feasibleModels, m)
		}
	}

	features := ExtractFeatures(req)
	candidates := make([]candidateAdaptiveEval, len(feasibleModels))

	// 2. Compute Empirical Predictions for each Feasible Model
	for i, m := range feasibleModels {
		key := ComputeFeatureKey(req, m.ID)
		var profile PerformanceProfile
		var lookupLevel string

		if store != nil {
			profile, lookupLevel = store.GetHierarchicalProfile(key)
		} else {
			profile = PerformanceProfile{Key: key}
			lookupLevel = "nominal_prior"
		}

		nominalQ := router.GetTaskQuality(m, req.TaskComplexity)
		pSuccess, quantiles := PredictSuccess(profile, prior)
		qEst := PredictQuality(profile, nominalQ)
		lEst := PredictLatency(m, profile, req.EstimatedInputTokens, req.EstimatedOutputTokens)
		cEst := PredictCost(m, req.EstimatedInputTokens, req.EstimatedOutputTokens, profile)

		n := float64(profile.TotalAttempts)
		confidence := n / (n + 10.0)

		candidates[i] = candidateAdaptiveEval{
			model:            m,
			features:         features,
			profile:          profile,
			lookupLevel:      lookupLevel,
			predictedSuccess: pSuccess,
			successQuantiles: quantiles,
			qualityEstimate:  qEst,
			latencyEstimate:  lEst,
			costEstimate:     cEst,
			confidence:       confidence,
		}
	}

	// 3. Normalization & Multi-Objective Expected Utility Scoring
	scoreAdaptiveCandidates(req, candidates, detector)

	// 4. Deterministic Sort & Tie-Breaking
	sort.SliceStable(candidates, func(i, j int) bool {
		if math.Abs(candidates[i].effectiveUtility-candidates[j].effectiveUtility) > 1e-6 {
			return candidates[i].effectiveUtility > candidates[j].effectiveUtility
		}
		if math.Abs(candidates[i].qualityEstimate.Mean-candidates[j].qualityEstimate.Mean) > 1e-6 {
			return candidates[i].qualityEstimate.Mean > candidates[j].qualityEstimate.Mean
		}
		if math.Abs(candidates[i].latencyEstimate.PredictedMs-candidates[j].latencyEstimate.PredictedMs) > 1e-6 {
			return candidates[i].latencyEstimate.PredictedMs < candidates[j].latencyEstimate.PredictedMs
		}
		return candidates[i].model.ID < candidates[j].model.ID
	})

	best := candidates[0]
	fallbacks := make([]string, 0, len(candidates)-1)
	for i := 1; i < len(candidates); i++ {
		fallbacks = append(fallbacks, candidates[i].model.ID)
	}

	return AdaptiveRoutingDecision{
		TaskID:               req.TaskID,
		RunID:                req.RunID,
		SelectedModelID:      best.model.ID,
		SelectedTier:         best.model.Tier,
		Policy:               req.RoutingPolicy,
		EffectiveUtility:     best.effectiveUtility,
		Confidence:           best.confidence,
		SampleCount:          best.profile.TotalAttempts,
		PredictedSuccess:     best.predictedSuccess,
		SuccessQuantiles:     best.successQuantiles,
		QualityEstimate:      best.qualityEstimate,
		LatencyEstimate:      best.latencyEstimate,
		CostEstimate:         best.costEstimate,
		NominalScore:         best.nominalScore,
		AdaptiveScore:        best.adaptiveScore,
		ScoreBreakdown: router.ScoreBreakdown{
			Quality:     best.qualityEstimate.Mean,
			Cost:        best.costScore,
			Latency:     best.latencyScore,
			Reliability: best.predictedSuccess,
		},
		FallbackCandidates:   fallbacks,
		Rejections:           nominalDecision.Rejections,
		LearningModelVersion: LearningModelVersion,
		FeatureSchemaVersion: FeatureSchemaVersion,
		PriorVersion:         prior.Version,
		DriftDetectorVersion: DriftDetectorVersion,
		DecidedAt:            time.Now().UTC(),
	}, nil
}

func scoreAdaptiveCandidates(req router.RoutingRequest, candidates []candidateAdaptiveEval, detector *DualWindowDriftDetector) {
	if len(candidates) == 0 {
		return
	}

	minCost := math.MaxFloat64
	maxCost := 0.0
	minLat := math.MaxFloat64
	maxLat := 0.0

	for _, c := range candidates {
		if c.costEstimate.PredictedUSD < minCost {
			minCost = c.costEstimate.PredictedUSD
		}
		if c.costEstimate.PredictedUSD > maxCost {
			maxCost = c.costEstimate.PredictedUSD
		}
		if c.latencyEstimate.PredictedMs < minLat {
			minLat = c.latencyEstimate.PredictedMs
		}
		if c.latencyEstimate.PredictedMs > maxLat {
			maxLat = c.latencyEstimate.PredictedMs
		}
	}

	costSpan := maxCost - minCost
	latSpan := maxLat - minLat

	wq, wc, wl, wr := getAdaptivePolicyWeights(req.RoutingPolicy)

	for i := range candidates {
		costScore := 1.0
		if costSpan > 1e-6 {
			costScore = 1.0 - (candidates[i].costEstimate.PredictedUSD-minCost)/costSpan
		}
		candidates[i].costScore = costScore

		latScore := 1.0
		if latSpan > 1e-6 {
			latScore = 1.0 - (candidates[i].latencyEstimate.PredictedMs-minLat)/latSpan
		}
		candidates[i].latencyScore = latScore

		// Drift penalty
		driftPenalty := 0.0
		if detector != nil {
			driftPenalty = detector.GetDriftPenalty(candidates[i].model.ID)
		}

		rawAdaptiveScore := wq*candidates[i].qualityEstimate.Mean +
			wc*costScore +
			wl*latScore +
			wr*candidates[i].predictedSuccess -
			driftPenalty

		candidates[i].adaptiveScore = math.Max(0.0, rawAdaptiveScore)

		// Nominal Stage 17 Score
		nomCost := router.EstimateCost(candidates[i].model, req.EstimatedInputTokens, req.EstimatedOutputTokens)
		nomLat := router.EstimateLatency(candidates[i].model, req.EstimatedInputTokens, req.EstimatedOutputTokens)
		nomQ := router.GetTaskQuality(candidates[i].model, req.TaskComplexity)

		nomCostScore := 1.0
		if costSpan > 1e-6 {
			nomCostScore = 1.0 - (nomCost-minCost)/costSpan
		}
		nomLatScore := 1.0
		if latSpan > 1e-6 {
			nomLatScore = 1.0 - (nomLat-minLat)/latSpan
		}

		candidates[i].nominalScore = wq*nomQ + wc*nomCostScore + wl*nomLatScore + wr*1.0

		// Confidence-weighted Bayesian Blend
		gamma := candidates[i].confidence
		candidates[i].effectiveUtility = gamma*candidates[i].adaptiveScore + (1.0-gamma)*candidates[i].nominalScore
	}
}

func getAdaptivePolicyWeights(policy router.RoutingPolicy) (wq, wc, wl, wr float64) {
	switch policy {
	case router.PolicyCostOptimized:
		return 0.10, 0.85, 0.00, 0.05
	case router.PolicyLatencyOptimized:
		return 0.10, 0.00, 0.85, 0.05
	case router.PolicyQualityOptimized:
		return 0.90, 0.00, 0.05, 0.05
	case router.PolicyBalanced:
		fallthrough
	default:
		return 0.50, 0.25, 0.20, 0.05
	}
}
