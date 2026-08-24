package onlinepolicy

import (
	"math"
	"time"

	"github.com/sentinelmesh/sentinelmesh/internal/router"
)

// CalculateReward evaluates the empirical scalar reward achieved by a model execution outcome.
func CalculateReward(outcome router.RoutingOutcomeRecord, req router.RoutingRequest, weights RewardWeights) float64 {
	// 1. Output Quality Component [0.0, 1.0]
	q := math.Max(0.0, math.Min(1.0, outcome.ActualQualityScore))

	// 2. Execution Reliability Success Indicator (succeeds only if API returns OK AND quality meets requirement)
	s := 0.0
	if outcome.Success && (req.QualityRequirement <= 0 || outcome.ActualQualityScore >= req.QualityRequirement) {
		s = 1.0
	}

	// 3. Cost Component Normalized against Budget
	costBudget := req.CostBudgetUSD
	if costBudget <= 0 {
		costBudget = 0.02 // Standard $0.02 reference ceiling
	}
	cNorm := math.Min(1.0, outcome.ActualCostUSD/costBudget)

	// 4. Latency Component Normalized against SLA
	slaMs := req.LatencySLAMs
	if slaMs <= 0 {
		slaMs = 1000.0 // Standard 1000ms reference ceiling
	}
	latMs := float64(outcome.ActualLatency) / float64(time.Millisecond)
	lNorm := math.Min(1.0, latMs/slaMs)

	// 5. Fallback Invocation Penalty [0.0 or 1.0]
	f := 0.0
	if outcome.FallbackUsed {
		f = 1.0
	}

	// Composite Reward Calculation
	rawReward := weights.WeightQuality*q +
		weights.WeightSuccess*s -
		weights.WeightCost*cNorm -
		weights.WeightLatency*lNorm -
		weights.WeightFallback*f

	return math.Max(-1.0, math.Min(1.0, rawReward))
}
