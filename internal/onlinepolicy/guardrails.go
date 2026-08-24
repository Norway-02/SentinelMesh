package onlinepolicy

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/sentinelmesh/sentinelmesh/internal/router"
)

// GuardrailEnforcer continuously audits policy execution metrics against safety limits with hysteresis.
type GuardrailEnforcer struct {
	mu                 sync.RWMutex
	config             GuardrailConfig
	outcomes           []router.RoutingOutcomeRecord
	consecutiveHealthy int
	isBreached         bool
	breachedReason     string
}

// NewGuardrailEnforcer constructs a GuardrailEnforcer.
func NewGuardrailEnforcer(config GuardrailConfig) *GuardrailEnforcer {
	return &GuardrailEnforcer{
		config:   config,
		outcomes: make([]router.RoutingOutcomeRecord, 0, config.RollingWindowSize),
	}
}

// RecordOutcome records an outcome and evaluates guardrail compliance.
func (g *GuardrailEnforcer) RecordOutcome(outcome router.RoutingOutcomeRecord, baselineCost, baselineLatMs float64) (bool, RollbackEvent) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.outcomes = append(g.outcomes, outcome)
	if len(g.outcomes) > g.config.RollingWindowSize {
		g.outcomes = g.outcomes[1:]
	}

	if len(g.outcomes) < 15 {
		// Minimum samples before triggering guardrail actions
		return false, RollbackEvent{}
	}

	// 1. Calculate Mean Quality
	sumQ := 0.0
	for _, o := range g.outcomes {
		sumQ += o.ActualQualityScore
	}
	meanQ := sumQ / float64(len(g.outcomes))

	// 2. Calculate Mean Cost
	sumC := 0.0
	for _, o := range g.outcomes {
		sumC += o.ActualCostUSD
	}
	meanC := sumC / float64(len(g.outcomes))

	// 3. Calculate P95 Latency
	lats := make([]float64, len(g.outcomes))
	for i, o := range g.outcomes {
		lats[i] = float64(o.ActualLatency) / float64(time.Millisecond)
	}
	sort.Float64s(lats)
	p95Lat := lats[int(float64(len(lats)-1)*0.95)]

	// 4. Calculate Fallback Rate
	fallbacks := 0
	for _, o := range g.outcomes {
		if o.FallbackUsed {
			fallbacks++
		}
	}
	fallbackRatePct := (float64(fallbacks) / float64(len(g.outcomes))) * 100.0

	// Check Breaches
	if !g.isBreached {
		if meanQ < g.config.MinQualityFloor {
			g.isBreached = true
			g.consecutiveHealthy = 0
			g.breachedReason = fmt.Sprintf("Quality floor breached: %.2f < %.2f", meanQ, g.config.MinQualityFloor)
			return true, RollbackEvent{
				TriggerMetric:  "quality_floor",
				ObservedValue:  meanQ,
				ThresholdValue: g.config.MinQualityFloor,
				Reason:         g.breachedReason,
				RolledBackAt:   time.Now().UTC(),
			}
		}

		if baselineCost > 0 {
			costIncreasePct := ((meanC - baselineCost) / baselineCost) * 100.0
			if costIncreasePct > g.config.MaxCostIncreasePct {
				g.isBreached = true
				g.consecutiveHealthy = 0
				g.breachedReason = fmt.Sprintf("Cost ceiling breached: +%.1f%% > +%.1f%%", costIncreasePct, g.config.MaxCostIncreasePct)
				return true, RollbackEvent{
					TriggerMetric:  "cost_ceiling",
					ObservedValue:  costIncreasePct,
					ThresholdValue: g.config.MaxCostIncreasePct,
					Reason:         g.breachedReason,
					RolledBackAt:   time.Now().UTC(),
				}
			}
		}

		if baselineLatMs > 0 {
			latIncreasePct := ((p95Lat - baselineLatMs) / baselineLatMs) * 100.0
			if latIncreasePct > g.config.MaxLatencyIncreasePct {
				g.isBreached = true
				g.consecutiveHealthy = 0
				g.breachedReason = fmt.Sprintf("Latency ceiling breached: +%.1f%% > +%.1f%%", latIncreasePct, g.config.MaxLatencyIncreasePct)
				return true, RollbackEvent{
					TriggerMetric:  "latency_ceiling",
					ObservedValue:  latIncreasePct,
					ThresholdValue: g.config.MaxLatencyIncreasePct,
					Reason:         g.breachedReason,
					RolledBackAt:   time.Now().UTC(),
				}
			}
		}

		if fallbackRatePct > g.config.MaxFallbackRatePct {
			g.isBreached = true
			g.consecutiveHealthy = 0
			g.breachedReason = fmt.Sprintf("Fallback rate breached: %.1f%% > %.1f%%", fallbackRatePct, g.config.MaxFallbackRatePct)
			return true, RollbackEvent{
				TriggerMetric:  "fallback_rate",
				ObservedValue:  fallbackRatePct,
				ThresholdValue: g.config.MaxFallbackRatePct,
				Reason:         g.breachedReason,
				RolledBackAt:   time.Now().UTC(),
			}
		}
	} else {
		// Hysteresis Recovery Check
		if meanQ >= g.config.QualityRecoveryFloor && fallbackRatePct <= 2.0 {
			g.consecutiveHealthy++
			if g.consecutiveHealthy >= g.config.RecoveryConsecutiveReq {
				g.isBreached = false
				g.breachedReason = ""
				g.consecutiveHealthy = 0
			}
		} else {
			g.consecutiveHealthy = 0
		}
	}

	return false, RollbackEvent{}
}

// IsBreached returns whether guardrails are currently in a breached state.
func (g *GuardrailEnforcer) IsBreached() (bool, string) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.isBreached, g.breachedReason
}

// Reset clears the state of the enforcer.
func (g *GuardrailEnforcer) Reset() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.outcomes = make([]router.RoutingOutcomeRecord, 0, g.config.RollingWindowSize)
	g.consecutiveHealthy = 0
	g.isBreached = false
	g.breachedReason = ""
}
