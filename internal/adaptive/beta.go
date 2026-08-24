package adaptive

import (
	"math"
)

// BetaQuantile computes the x in [0, 1] such that CDF(Beta(a, b), x) = p.
func BetaQuantile(a, b, p float64) float64 {
	if p <= 0.0 {
		return 0.0
	}
	if p >= 1.0 {
		return 1.0
	}
	if a <= 0.0 || b <= 0.0 {
		return p
	}

	// Beta distribution mean and variance
	mean := a / (a + b)
	variance := (a * b) / ((a + b) * (a + b) * (a + b + 1.0))
	stdDev := math.Sqrt(variance)

	// For a+b >= 4.0, Cornish-Fisher expansion for Beta is extremely fast and accurate
	if a+b >= 4.0 {
		// Standard normal inverse quantile
		z := standardNormalQuantile(p)
		// Cornish-Fisher expansion for Beta
		skewness := 2.0 * (b - a) * math.Sqrt(a+b+1.0) / ((a + b + 2.0) * math.Sqrt(a*b))
		w := z + (z*z-1.0)*skewness/6.0
		val := mean + w*stdDev
		return math.Max(0.001, math.Min(0.999, val))
	}

	// Binary search with Simpson integration for small a, b
	low := 0.0
	high := 1.0
	for iter := 0; iter < 30; iter++ {
		mid := (low + high) / 2.0
		cdf := betaCDFSimpson(a, b, mid)
		if cdf < p {
			low = mid
		} else {
			high = mid
		}
	}
	return (low + high) / 2.0
}

// ComputeBetaQuantiles calculates 2.5%, 50% (median), and 97.5% quantiles.
func ComputeBetaQuantiles(a, b float64) BetaQuantiles {
	return BetaQuantiles{
		Q025: BetaQuantile(a, b, 0.025),
		Q50:  BetaQuantile(a, b, 0.500),
		Q975: BetaQuantile(a, b, 0.975),
	}
}

func betaCDFSimpson(a, b, x float64) float64 {
	if x <= 0.0 {
		return 0.0
	}
	if x >= 1.0 {
		return 1.0
	}

	lgammaA, _ := math.Lgamma(a)
	lgammaB, _ := math.Lgamma(b)
	lgammaAB, _ := math.Lgamma(a + b)
	lnBeta := lgammaA + lgammaB - lgammaAB

	// Simpson numerical integration
	const n = 100
	h := x / float64(n)
	sum := 0.0

	for i := 0; i <= n; i++ {
		t := float64(i) * h
		if t <= 0.0 || t >= 1.0 {
			continue
		}
		y := math.Exp((a-1.0)*math.Log(t) + (b-1.0)*math.Log(1.0-t) - lnBeta)
		weight := 2.0
		if i == 0 || i == n {
			weight = 1.0
		} else if i%2 == 1 {
			weight = 4.0
		}
		sum += weight * y
	}

	res := (h / 3.0) * sum
	return math.Max(0.0, math.Min(1.0, res))
}

// standardNormalQuantile approximates the inverse CDF of the standard normal distribution (rational approximation).
func standardNormalQuantile(p float64) float64 {
	if p <= 0.0 {
		return -5.0
	}
	if p >= 1.0 {
		return 5.0
	}
	if p == 0.5 {
		return 0.0
	}

	// Abramowitz and Stegun approximation (formula 26.2.23)
	q := p
	sign := 1.0
	if p < 0.5 {
		q = p
		sign = -1.0
	} else {
		q = 1.0 - p
		sign = 1.0
	}

	t := math.Sqrt(-2.0 * math.Log(q))
	c0 := 2.515517
	c1 := 0.802853
	c2 := 0.010328
	d1 := 1.432788
	d2 := 0.189269
	d3 := 0.001308

	numerator := c0 + c1*t + c2*t*t
	denominator := 1.0 + d1*t + d2*t*t + d3*t*t*t

	z := t - (numerator / denominator)
	return sign * z
}
