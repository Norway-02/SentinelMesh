package adaptive

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

const (
	BaselineWindowSize = 100
	RecentWindowSize   = 20
	MinRecentSamples   = 10

	QualityDropThreshold   = 0.15 // 15% absolute drop
	LatencySpikeThreshold  = 0.80 // 80% increase in P95
	FailureSpikeThreshold  = 0.20 // 20% increase in failure rate
	DriftPenaltyScoreScale = 0.35 // Penalty applied to degraded models
)

type observation struct {
	quality   float64
	latencyMs float64
	success   bool
	timestamp time.Time
}

type modelWindow struct {
	mu           sync.RWMutex
	modelID      string
	isDegraded   bool
	degradedAt   time.Time
	degradeReason string
	observations []observation
}

// DualWindowDriftDetector monitors live model endpoint execution telemetry.
type DualWindowDriftDetector struct {
	mu      sync.RWMutex
	windows map[string]*modelWindow
}

// NewDualWindowDriftDetector creates a new drift detector.
func NewDualWindowDriftDetector() *DualWindowDriftDetector {
	return &DualWindowDriftDetector{
		windows: make(map[string]*modelWindow),
	}
}

// RecordObservation adds an observation and evaluates statistical drift.
func (d *DualWindowDriftDetector) RecordObservation(modelID string, quality float64, latencyMs float64, success bool) (bool, string, string) {
	w := d.getOrCreateWindow(modelID)
	w.mu.Lock()
	defer w.mu.Unlock()

	w.observations = append(w.observations, observation{
		quality:   quality,
		latencyMs: latencyMs,
		success:   success,
		timestamp: time.Now().UTC(),
	})

	if len(w.observations) > BaselineWindowSize+RecentWindowSize {
		w.observations = w.observations[1:]
	}

	total := len(w.observations)
	if total < RecentWindowSize+MinRecentSamples {
		return false, "", ""
	}

	recent := w.observations[total-RecentWindowSize:]
	baseline := w.observations[:total-RecentWindowSize]

	// 1. Calculate Quality Means
	baseQ := meanQuality(baseline)
	recQ := meanQuality(recent)
	deltaQ := baseQ - recQ

	// 2. Calculate Latency P95s
	baseP95 := p95Latency(baseline)
	recP95 := p95Latency(recent)
	deltaL := 0.0
	if baseP95 > 0 {
		deltaL = (recP95 - baseP95) / baseP95
	}

	// 3. Calculate Failure Rates
	baseFailRate := failureRate(baseline)
	recFailRate := failureRate(recent)
	deltaF := recFailRate - baseFailRate

	// Evaluation
	if !w.isDegraded {
		if deltaQ >= QualityDropThreshold {
			w.isDegraded = true
			w.degradedAt = time.Now().UTC()
			w.degradeReason = fmt.Sprintf("Quality dropped from %.2f to %.2f (Δ=%.2f)", baseQ, recQ, deltaQ)
			return true, "quality_drop", w.degradeReason
		}
		if deltaL >= LatencySpikeThreshold {
			w.isDegraded = true
			w.degradedAt = time.Now().UTC()
			w.degradeReason = fmt.Sprintf("P95 latency increased from %.1fms to %.1fms (+%.1f%%)", baseP95, recP95, deltaL*100.0)
			return true, "latency_spike", w.degradeReason
		}
		if deltaF >= FailureSpikeThreshold {
			w.isDegraded = true
			w.degradedAt = time.Now().UTC()
			w.degradeReason = fmt.Sprintf("Failure rate spiked from %.1f%% to %.1f%% (+%.1f%%)", baseFailRate*100, recFailRate*100, deltaF*100)
			return true, "failure_spike", w.degradeReason
		}
	} else {
		// Recovery check
		if deltaQ < 0.05 && deltaL < 0.20 && deltaF < 0.05 {
			w.isDegraded = false
			w.degradeReason = ""
		}
	}

	return false, "", ""
}

// IsDegraded checks if a model endpoint is currently flagged as degraded.
func (d *DualWindowDriftDetector) IsDegraded(modelID string) (bool, string) {
	w := d.getOrCreateWindow(modelID)
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.isDegraded, w.degradeReason
}

// GetDriftPenalty returns utility penalty if model is degraded.
func (d *DualWindowDriftDetector) GetDriftPenalty(modelID string) float64 {
	isDegraded, _ := d.IsDegraded(modelID)
	if isDegraded {
		return DriftPenaltyScoreScale
	}
	return 0.0
}

func (d *DualWindowDriftDetector) getOrCreateWindow(modelID string) *modelWindow {
	d.mu.Lock()
	defer d.mu.Unlock()

	w, ok := d.windows[modelID]
	if !ok {
		w = &modelWindow{
			modelID:      modelID,
			observations: make([]observation, 0, BaselineWindowSize+RecentWindowSize),
		}
		d.windows[modelID] = w
	}
	return w
}

func meanQuality(obs []observation) float64 {
	if len(obs) == 0 {
		return 0.0
	}
	sum := 0.0
	for _, o := range obs {
		sum += o.quality
	}
	return sum / float64(len(obs))
}

func p95Latency(obs []observation) float64 {
	if len(obs) == 0 {
		return 0.0
	}
	lats := make([]float64, len(obs))
	for i, o := range obs {
		lats[i] = o.latencyMs
	}
	sort.Float64s(lats)
	idx := int(float64(len(lats)-1) * 0.95)
	return lats[idx]
}

func failureRate(obs []observation) float64 {
	if len(obs) == 0 {
		return 0.0
	}
	failures := 0
	for _, o := range obs {
		if !o.success {
			failures++
		}
	}
	return float64(failures) / float64(len(obs))
}
