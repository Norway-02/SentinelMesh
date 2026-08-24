package router

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

const (
	AlgorithmVersion = "router-v1.0"
	PolicyVersion    = "policy-v1.0"
)

type candidateEvaluation struct {
	model            ModelDefinition
	qualityScore     float64
	estimatedCost    float64
	estimatedLatency float64
	reliabilityScore float64
	costScore        float64
	latencyScore     float64
	finalScore       float64
	isParetoOptimal  bool
}

// RouteTask evaluates registered models against a task request deterministically.
func RouteTask(req RoutingRequest, models []ModelDefinition) (RoutingDecision, error) {
	if len(models) == 0 {
		return RoutingDecision{}, fmt.Errorf("no models registered in router")
	}

	regVersion := req.RegistryVersion
	if regVersion == "" {
		regVersion = "registry-v1.0"
	}
	polVersion := req.PolicyVersion
	if polVersion == "" {
		polVersion = PolicyVersion
	}

	// 1. Static Pin Shortcut
	if req.RoutingPolicy == PolicyStatic && req.PinnedModelID != "" {
		var rejections []ModelRejection
		for _, m := range models {
			if m.ID == req.PinnedModelID {
				cost := EstimateCost(m, req.EstimatedInputTokens, req.EstimatedOutputTokens)
				lat := EstimateLatency(m, req.EstimatedInputTokens, req.EstimatedOutputTokens)
				q := GetTaskQuality(m, req.TaskComplexity)
				return RoutingDecision{
					TaskID:             req.TaskID,
					RunID:              req.RunID,
					SelectedModelID:    m.ID,
					SelectedTier:       m.Tier,
					Policy:             PolicyStatic,
					AlgorithmVersion:   AlgorithmVersion,
					RegistryVersion:    regVersion,
					PolicyVersion:      polVersion,
					EstimatedCostUSD:   cost,
					EstimatedLatencyMs: lat,
					QualityScore:       q,
					FinalScore:         1.0,
					ScoreBreakdown: ScoreBreakdown{
						Quality:     q,
						Cost:        1.0,
						Latency:     1.0,
						Reliability: 1.0 - m.ObservedMetrics.ObservedErrorRate,
					},
					Rejections:      rejections,
					IsParetoOptimal: true,
					DecidedAt:       time.Now().UTC(),
				}, nil
			} else {
				rejections = append(rejections, ModelRejection{
					ModelID: m.ID,
					Reason:  "unpinned_model",
					Details: fmt.Sprintf("Model %s was rejected because static policy pinned %s", m.ID, req.PinnedModelID),
				})
			}
		}
		return RoutingDecision{}, fmt.Errorf("pinned model %s not found", req.PinnedModelID)
	}

	// 2. Hard Constraint Filtering & Rejection Auditing
	feasible, rejections := filterFeasibleModels(req, models)
	if len(feasible) == 0 {
		return RoutingDecision{
			TaskID:           req.TaskID,
			RunID:            req.RunID,
			Policy:           req.RoutingPolicy,
			AlgorithmVersion: AlgorithmVersion,
			RegistryVersion:  regVersion,
			PolicyVersion:    polVersion,
			Rejections:       rejections,
			DecidedAt:        time.Now().UTC(),
		}, fmt.Errorf("no feasible models satisfy hard constraints (security, context, health, quality threshold)")
	}

	// 3. Compute Pareto Optimality across feasible candidates
	markParetoFrontier(feasible)

	// 4. Safe Normalization & Multi-Objective Scoring
	scored := scoreFeasibleCandidates(req, feasible)

	// 5. Deterministic Sort & Tie-Breaking
	sortCandidates(scored)

	best := scored[0]
	fallbacks := make([]string, 0, len(scored)-1)
	for i := 1; i < len(scored); i++ {
		fallbacks = append(fallbacks, scored[i].model.ID)
	}

	return RoutingDecision{
		TaskID:             req.TaskID,
		RunID:              req.RunID,
		SelectedModelID:    best.model.ID,
		SelectedTier:       best.model.Tier,
		Policy:             req.RoutingPolicy,
		AlgorithmVersion:   AlgorithmVersion,
		RegistryVersion:    regVersion,
		PolicyVersion:      polVersion,
		EstimatedCostUSD:   best.estimatedCost,
		EstimatedLatencyMs: best.estimatedLatency,
		QualityScore:       best.qualityScore,
		FinalScore:         best.finalScore,
		ScoreBreakdown: ScoreBreakdown{
			Quality:     best.qualityScore,
			Cost:        best.costScore,
			Latency:     best.latencyScore,
			Reliability: best.reliabilityScore,
		},
		FallbackCandidates: fallbacks,
		Rejections:         rejections,
		IsParetoOptimal:    best.isParetoOptimal,
		DecidedAt:          time.Now().UTC(),
	}, nil
}

// Replay reproduces a historical routing decision given identical inputs.
func Replay(req RoutingRequest, models []ModelDefinition) (RoutingDecision, error) {
	return RouteTask(req, models)
}

func filterFeasibleModels(req RoutingRequest, models []ModelDefinition) ([]candidateEvaluation, []ModelRejection) {
	var feasible []candidateEvaluation
	var rejections []ModelRejection
	totalTokens := req.EstimatedInputTokens + req.EstimatedOutputTokens

	for _, m := range models {
		// Filter 1: Security compatibility (Strict Hard Constraint)
		if !supportsSecurityProfile(m.SecurityClasses, req.SecurityProfile) {
			rejections = append(rejections, ModelRejection{
				ModelID: m.ID,
				Reason:  "incompatible_security_profile",
				Details: fmt.Sprintf("Model security classes %v do not permit required profile '%s'", m.SecurityClasses, req.SecurityProfile),
			})
			continue
		}

		// Filter 2: Context capacity
		if m.ContextWindow < totalTokens {
			rejections = append(rejections, ModelRejection{
				ModelID: m.ID,
				Reason:  "context_capacity_exceeded",
				Details: fmt.Sprintf("Estimated tokens (%d) exceed model context window (%d)", totalTokens, m.ContextWindow),
			})
			continue
		}

		// Filter 3: Availability (Circuit Breaker status)
		if m.HealthStatus == HealthUnavailable {
			rejections = append(rejections, ModelRejection{
				ModelID: m.ID,
				Reason:  "circuit_breaker_unavailable",
				Details: fmt.Sprintf("Model endpoint is currently UNAVAILABLE (circuit breaker open) with error rate %.2f", m.ObservedMetrics.ObservedErrorRate),
			})
			continue
		}

		// Filter 4: Task Complexity Empirical Quality
		quality := GetTaskQuality(m, req.TaskComplexity)
		if req.QualityRequirement > 0 && quality < req.QualityRequirement {
			rejections = append(rejections, ModelRejection{
				ModelID: m.ID,
				Reason:  "quality_below_threshold",
				Details: fmt.Sprintf("Model quality on %s (%.2f) is below required threshold (%.2f)", req.TaskComplexity, quality, req.QualityRequirement),
			})
			continue
		}

		// Cost & Latency Estimation
		cost := EstimateCost(m, req.EstimatedInputTokens, req.EstimatedOutputTokens)
		lat := EstimateLatency(m, req.EstimatedInputTokens, req.EstimatedOutputTokens)

		// Filter 5: Cost Budget constraint
		if req.CostBudgetUSD > 0 && cost > req.CostBudgetUSD {
			rejections = append(rejections, ModelRejection{
				ModelID: m.ID,
				Reason:  "cost_budget_exceeded",
				Details: fmt.Sprintf("Estimated cost ($%.5f) exceeds budget ($%.5f)", cost, req.CostBudgetUSD),
			})
			continue
		}

		// Filter 6: Latency SLA constraint
		if req.LatencySLAMs > 0 && lat > req.LatencySLAMs {
			rejections = append(rejections, ModelRejection{
				ModelID: m.ID,
				Reason:  "latency_sla_exceeded",
				Details: fmt.Sprintf("Estimated latency (%.1fms) exceeds SLA (%.1fms)", lat, req.LatencySLAMs),
			})
			continue
		}

		reliability := 1.0 - m.ObservedMetrics.ObservedErrorRate
		if reliability < 0.0 {
			reliability = 0.0
		}

		feasible = append(feasible, candidateEvaluation{
			model:            m,
			qualityScore:     quality,
			estimatedCost:    cost,
			estimatedLatency: lat,
			reliabilityScore: reliability,
		})
	}
	return feasible, rejections
}

func markParetoFrontier(candidates []candidateEvaluation) {
	n := len(candidates)
	for i := 0; i < n; i++ {
		dominated := false
		for j := 0; j < n; j++ {
			if i == j {
				continue
			}
			// Candidate j dominates candidate i if j has >= quality, <= cost, <= latency, with at least one strict improvement
			betterOrEqual := candidates[j].qualityScore >= candidates[i].qualityScore &&
				candidates[j].estimatedCost <= candidates[i].estimatedCost &&
				candidates[j].estimatedLatency <= candidates[i].estimatedLatency

			strictlyBetter := candidates[j].qualityScore > candidates[i].qualityScore ||
				candidates[j].estimatedCost < candidates[i].estimatedCost ||
				candidates[j].estimatedLatency < candidates[i].estimatedLatency

			if betterOrEqual && strictlyBetter {
				dominated = true
				break
			}
		}
		candidates[i].isParetoOptimal = !dominated
	}
}

func scoreFeasibleCandidates(req RoutingRequest, candidates []candidateEvaluation) []candidateEvaluation {
	if len(candidates) == 0 {
		return nil
	}

	minCost, maxCost := candidates[0].estimatedCost, candidates[0].estimatedCost
	minLat, maxLat := candidates[0].estimatedLatency, candidates[0].estimatedLatency

	for _, c := range candidates {
		if c.estimatedCost < minCost {
			minCost = c.estimatedCost
		}
		if c.estimatedCost > maxCost {
			maxCost = c.estimatedCost
		}
		if c.estimatedLatency < minLat {
			minLat = c.estimatedLatency
		}
		if c.estimatedLatency > maxLat {
			maxLat = c.estimatedLatency
		}
	}

	wq, wc, wl, wr := getPolicyWeights(req.RoutingPolicy)

	for i := range candidates {
		c := &candidates[i]

		// Safe min-max normalization
		if maxCost > minCost {
			normCost := (c.estimatedCost - minCost) / (maxCost - minCost)
			c.costScore = 1.0 - normCost
		} else {
			c.costScore = 1.0
		}

		if maxLat > minLat {
			normLat := (c.estimatedLatency - minLat) / (maxLat - minLat)
			c.latencyScore = 1.0 - normLat
		} else {
			c.latencyScore = 1.0
		}

		c.finalScore = (wq * c.qualityScore) + (wc * c.costScore) + (wl * c.latencyScore) + (wr * c.reliabilityScore)
	}

	return candidates
}

func sortCandidates(candidates []candidateEvaluation) {
	sort.Slice(candidates, func(i, j int) bool {
		// 1. Primary: FinalScore DESC
		if math.Abs(candidates[i].finalScore-candidates[j].finalScore) > 1e-6 {
			return candidates[i].finalScore > candidates[j].finalScore
		}
		// 2. Secondary: QualityScore DESC
		if math.Abs(candidates[i].qualityScore-candidates[j].qualityScore) > 1e-6 {
			return candidates[i].qualityScore > candidates[j].qualityScore
		}
		// 3. Tertiary: EstimatedLatency ASC
		if math.Abs(candidates[i].estimatedLatency-candidates[j].estimatedLatency) > 1e-6 {
			return candidates[i].estimatedLatency < candidates[j].estimatedLatency
		}
		// 4. Quaternary: ModelID ASC (Strict Deterministic Tie-Breaker)
		return candidates[i].model.ID < candidates[j].model.ID
	})
}

func getPolicyWeights(policy RoutingPolicy) (wq, wc, wl, wr float64) {
	switch policy {
	case PolicyCostOptimized:
		return 0.10, 0.85, 0.00, 0.05
	case PolicyLatencyOptimized:
		return 0.10, 0.00, 0.85, 0.05
	case PolicyQualityOptimized:
		return 0.90, 0.00, 0.05, 0.05
	case PolicyBalanced:
		fallthrough
	default:
		return 0.50, 0.25, 0.20, 0.05
	}
}

// EstimateCost calculates USD cost for given token counts.
func EstimateCost(m ModelDefinition, inputTokens, outputTokens int) float64 {
	inCost := (float64(inputTokens) / 1000.0) * m.CostPer1kInputTokens
	outCost := (float64(outputTokens) / 1000.0) * m.CostPer1kOutputTokens
	return inCost + outCost
}

// EstimateLatency models execution latency based on base overhead and token processing rates.
func EstimateLatency(m ModelDefinition, inputTokens, outputTokens int) float64 {
	inLat := (float64(inputTokens) / 1000.0) * m.InputMsPer1kTokens
	outLat := (float64(outputTokens) / 1000.0) * m.OutputMsPer1kTokens
	return m.BaseOverheadMs + inLat + outLat
}

// GetTaskQuality extracts the empirical quality score for a given complexity class.
func GetTaskQuality(m ModelDefinition, complexity TaskComplexity) float64 {
	if m.TaskQualityMatrix != nil {
		if q, ok := m.TaskQualityMatrix[complexity]; ok {
			return q
		}
	}
	// Fallback conservative defaults by tier
	switch m.Tier {
	case TierSmall:
		return 0.65
	case TierMedium:
		return 0.85
	case TierLarge:
		return 0.95
	default:
		return 0.50
	}
}

func supportsSecurityProfile(allowedClasses []string, requiredProfile string) bool {
	if requiredProfile == "" || requiredProfile == "standard" || requiredProfile == "public" {
		return true
	}
	for _, c := range allowedClasses {
		if strings.EqualFold(c, requiredProfile) {
			return true
		}
	}
	return false
}
