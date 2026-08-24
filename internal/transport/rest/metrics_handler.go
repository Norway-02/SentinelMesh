package rest

import (
	"encoding/json"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/sentinelmesh/sentinelmesh/internal/router"
)

// MetricsSummary is the aggregated dashboard metric snapshot.
type MetricsSummary struct {
	ActiveTasks        int       `json:"active_tasks"`
	TotalDecisions     int       `json:"total_decisions"`
	SuccessRate        float64   `json:"success_rate"`
	MeanLatencyMs      float64   `json:"mean_latency_ms"`
	P95LatencyMs       float64   `json:"p95_latency_ms"`
	TotalCostUSD       float64   `json:"total_cost_usd"`
	FallbackRate       float64   `json:"fallback_rate"`
	DriftAlerts        int       `json:"drift_alerts"`
	PolicyRollbacks    int       `json:"policy_rollbacks"`
	ActiveProviders    int       `json:"active_providers"`
	RequestsPerMinute  float64   `json:"requests_per_minute"`
	CalculatedAt       time.Time `json:"calculated_at"`
}

// MetricsHandler computes aggregated metrics from routing outcomes.
type MetricsHandler struct {
	decisionRepo    router.DecisionRepository
	registry        router.Registry
	mu              sync.Mutex
	driftAlerts     int
	policyRollbacks int
}

// NewMetricsHandler creates a MetricsHandler.
func NewMetricsHandler(decisionRepo router.DecisionRepository, registry router.Registry) *MetricsHandler {
	return &MetricsHandler{
		decisionRepo: decisionRepo,
		registry:     registry,
	}
}

// RegisterRoutes registers metrics endpoints.
func (h *MetricsHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/metrics/summary", h.Summary)
}

// IncrementDriftAlerts increments the drift alert counter (called by SSE hub or adaptive service).
func (h *MetricsHandler) IncrementDriftAlerts() {
	h.mu.Lock()
	h.driftAlerts++
	h.mu.Unlock()
}

// IncrementPolicyRollbacks increments the rollback counter.
func (h *MetricsHandler) IncrementPolicyRollbacks() {
	h.mu.Lock()
	h.policyRollbacks++
	h.mu.Unlock()
}

// Summary handles GET /v1/metrics/summary
func (h *MetricsHandler) Summary(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	outcomes, err := h.decisionRepo.ListOutcomes(ctx, 500)
	if err != nil {
		outcomes = []router.RoutingOutcomeRecord{}
	}

	models, _ := h.registry.ListModels(ctx)
	activeProviders := 0
	for _, m := range models {
		if m.HealthStatus != router.HealthUnavailable {
			activeProviders++
		}
	}

	h.mu.Lock()
	driftAlerts := h.driftAlerts
	policyRollbacks := h.policyRollbacks
	h.mu.Unlock()

	summary := computeSummary(outcomes, activeProviders, driftAlerts, policyRollbacks)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(summary)
}

func computeSummary(outcomes []router.RoutingOutcomeRecord, activeProviders, driftAlerts, rollbacks int) MetricsSummary {
	if len(outcomes) == 0 {
		return MetricsSummary{
			ActiveProviders: activeProviders,
			DriftAlerts:     driftAlerts,
			PolicyRollbacks: rollbacks,
			CalculatedAt:    time.Now().UTC(),
		}
	}

	var totalCost float64
	var successCount int
	var fallbackCount int
	var latencies []float64
	var recentCount int

	cutoff := time.Now().Add(-1 * time.Minute)

	for _, o := range outcomes {
		totalCost += o.ActualCostUSD
		if o.Success {
			successCount++
		}
		if o.FallbackUsed {
			fallbackCount++
		}
		latMs := float64(o.ActualLatency) / float64(time.Millisecond)
		if latMs > 0 {
			latencies = append(latencies, latMs)
		}
		if o.CompletedAt.After(cutoff) {
			recentCount++
		}
	}

	total := len(outcomes)
	successRate := 0.0
	if total > 0 {
		successRate = float64(successCount) / float64(total)
	}

	fallbackRate := 0.0
	if total > 0 {
		fallbackRate = float64(fallbackCount) / float64(total)
	}

	meanLat := 0.0
	p95Lat := 0.0
	if len(latencies) > 0 {
		sum := 0.0
		for _, l := range latencies {
			sum += l
		}
		meanLat = sum / float64(len(latencies))

		sorted := make([]float64, len(latencies))
		copy(sorted, latencies)
		sort.Float64s(sorted)
		idx := int(float64(len(sorted)) * 0.95)
		if idx >= len(sorted) {
			idx = len(sorted) - 1
		}
		p95Lat = sorted[idx]
	}

	return MetricsSummary{
		ActiveTasks:       0, // No persistent active-task tracking yet
		TotalDecisions:    total,
		SuccessRate:       successRate,
		MeanLatencyMs:     meanLat,
		P95LatencyMs:      p95Lat,
		TotalCostUSD:      totalCost,
		FallbackRate:      fallbackRate,
		DriftAlerts:       driftAlerts,
		PolicyRollbacks:   rollbacks,
		ActiveProviders:   activeProviders,
		RequestsPerMinute: float64(recentCount),
		CalculatedAt:      time.Now().UTC(),
	}
}
