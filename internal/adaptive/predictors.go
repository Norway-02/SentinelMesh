package adaptive

import (
	"math"
	"sort"

	"github.com/sentinelmesh/sentinelmesh/internal/router"
)

const (
	QualityShrinkagePriorCount = 10.0
	CostCorrectionPriorCount   = 10.0
)

// PredictSuccess calculates the Bayesian posterior success probability and exact Beta quantiles.
func PredictSuccess(profile PerformanceProfile, prior BetaPrior) (float64, BetaQuantiles) {
	if prior.Alpha <= 0 {
		prior.Alpha = 5.0
	}
	if prior.Beta <= 0 {
		prior.Beta = 1.0
	}

	a := prior.Alpha + float64(profile.SuccessCount)
	b := prior.Beta + float64(profile.FailureCount)

	mean := a / (a + b)
	quantiles := ComputeBetaQuantiles(a, b)
	return mean, quantiles
}

// PredictQuality calculates empirical-Bayes shrunk quality and confidence interval.
func PredictQuality(profile PerformanceProfile, nominalQ float64) QualityEstimate {
	n := float64(profile.TotalAttempts)
	if n == 0 {
		return QualityEstimate{
			Mean:     nominalQ,
			Variance: 0.01,
			Samples:  0,
			LowerCI:  math.Max(0.0, nominalQ-0.10),
			UpperCI:  math.Min(1.0, nominalQ+0.10),
		}
	}

	observedMean := profile.QualitySum / n
	shrunkMean := (n/(n+QualityShrinkagePriorCount))*observedMean + (QualityShrinkagePriorCount/(n+QualityShrinkagePriorCount))*nominalQ
	shrunkMean = math.Max(0.0, math.Min(1.0, shrunkMean))

	// Sample variance calculation
	variance := 0.01
	if n > 1 {
		rawVar := (profile.QualitySqSum - (profile.QualitySum*profile.QualitySum)/n) / (n - 1.0)
		if rawVar > 0.0001 {
			variance = rawVar
		}
	}

	stdDev := math.Sqrt(variance)
	lowerCI := math.Max(0.0, shrunkMean-1.96*stdDev)
	upperCI := math.Min(1.0, shrunkMean+1.96*stdDev)

	return QualityEstimate{
		Mean:     shrunkMean,
		Variance: variance,
		Samples:  profile.TotalAttempts,
		LowerCI:  lowerCI,
		UpperCI:  upperCI,
	}
}

// PredictLatency computes linear regression expected latency alongside observed tail percentiles.
func PredictLatency(model router.ModelDefinition, profile PerformanceProfile, inTokens, outTokens int) LatencyEstimate {
	minLat := math.Max(1.0, model.NominalP50LatencyMs*0.20)
	n := profile.TotalAttempts

	var predictedMs float64
	if n >= 10 && profile.LatencyRegression.Theta0 > 0 {
		predictedMs = profile.LatencyRegression.Theta0 +
			profile.LatencyRegression.Theta1*float64(inTokens) +
			profile.LatencyRegression.Theta2*float64(outTokens)
	} else {
		// Fallback to nominal model specification
		predictedMs = router.EstimateLatency(model, inTokens, outTokens)
	}

	if predictedMs < minLat {
		predictedMs = minLat
	}

	// Compute rolling P50 and P95 from recent observations
	p50 := predictedMs
	p95 := predictedMs * 1.5

	if len(profile.RecentLatencies) > 0 {
		sorted := make([]float64, len(profile.RecentLatencies))
		copy(sorted, profile.RecentLatencies)
		sort.Float64s(sorted)

		p50Idx := int(float64(len(sorted)-1) * 0.50)
		p95Idx := int(float64(len(sorted)-1) * 0.95)
		p50 = sorted[p50Idx]
		p95 = sorted[p95Idx]
	}

	return LatencyEstimate{
		PredictedMs:   predictedMs,
		ObservedP50Ms: p50,
		ObservedP95Ms: p95,
		Samples:       n,
	}
}

// PredictCost applies an empirical correction ratio to deterministic token pricing.
func PredictCost(model router.ModelDefinition, inTokens, outTokens int, profile PerformanceProfile) CostEstimate {
	nominalCost := router.EstimateCost(model, inTokens, outTokens)
	n := float64(profile.TotalAttempts)

	correctionFactor := 1.0
	if n > 0 && profile.CostNominalSum > 0 {
		observedRatio := profile.CostActualSum / profile.CostNominalSum
		// Shrinkage towards 1.0
		correctionFactor = (n/(n+CostCorrectionPriorCount))*observedRatio + (CostCorrectionPriorCount/(n+CostCorrectionPriorCount))*1.0
	}

	predictedCost := nominalCost * correctionFactor
	if predictedCost < 0.0 {
		predictedCost = 0.0
	}

	return CostEstimate{
		PredictedUSD:     predictedCost,
		NominalUSD:       nominalCost,
		CorrectionFactor: correctionFactor,
		Samples:          profile.TotalAttempts,
	}
}
