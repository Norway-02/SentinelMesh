package rest

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/sentinelmesh/sentinelmesh/internal/router"
)

// ModelHandler exposes model registry information.
type ModelHandler struct {
	registry     router.Registry
	liveProvider *router.LiveModelProvider
}

// NewModelHandler creates a ModelHandler.
func NewModelHandler(registry router.Registry, liveProvider *router.LiveModelProvider) *ModelHandler {
	return &ModelHandler{
		registry:     registry,
		liveProvider: liveProvider,
	}
}

// RegisterRoutes registers model registry routes.
func (h *ModelHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/models", h.ListModels)
	mux.HandleFunc("GET /v1/models/{id}", h.GetModel)
}

// ModelResponse is the safe public subset of ModelDefinition exposed to the UI.
// Never includes API keys, credentials, or private secrets.
type ModelResponse struct {
	ID                    string                              `json:"id"`
	Name                  string                              `json:"name"`
	Tier                  string                              `json:"tier"`
	Provider              string                              `json:"provider"`
	ContextWindow         int                                 `json:"context_window"`
	SecurityClasses       []string                            `json:"security_classes"`
	CostPer1kInputTokens  float64                             `json:"cost_per_1k_input_tokens"`
	CostPer1kOutputTokens float64                             `json:"cost_per_1k_output_tokens"`
	NominalP50LatencyMs   float64                             `json:"nominal_p50_latency_ms"`
	NominalP95LatencyMs   float64                             `json:"nominal_p95_latency_ms"`
	TaskQualityMatrix     map[router.TaskComplexity]float64   `json:"task_quality_matrix"`
	HealthStatus          string                              `json:"health_status"`
	ObservedMetrics       router.ObservedModelMetrics         `json:"observed_metrics"`
}

// ListModels handles GET /v1/models
func (h *ModelHandler) ListModels(w http.ResponseWriter, r *http.Request) {
	models, err := h.registry.ListModels(r.Context())
	if err != nil {
		WriteError(w, err)
		return
	}

	isLive := h.liveProvider != nil && h.liveProvider.GetMode() == router.ModeLive

	resp := make([]ModelResponse, 0, len(models))
	for _, m := range models {
		if isLive && (m.Provider == "synthetic-cloud" || m.Provider == "synthetic-local" || m.Provider == "synthetic") {
			continue
		}
		resp = append(resp, toModelResponse(m))
	}

	// Sort by tier for consistent output: small → medium → large
	sortModels(resp)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"models":          resp,
		"count":           len(resp),
		"registry_version": h.registry.Version(),
	})
}

// GetModel handles GET /v1/models/{id}
func (h *ModelHandler) GetModel(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "model id required", http.StatusBadRequest)
		return
	}

	model, err := h.registry.GetModel(r.Context(), id)
	if err != nil {
		WriteError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(toModelResponse(model))
}

func toModelResponse(m router.ModelDefinition) ModelResponse {
	return ModelResponse{
		ID:                    m.ID,
		Name:                  m.Name,
		Tier:                  string(m.Tier),
		Provider:              m.Provider,
		ContextWindow:         m.ContextWindow,
		SecurityClasses:       m.SecurityClasses,
		CostPer1kInputTokens:  m.CostPer1kInputTokens,
		CostPer1kOutputTokens: m.CostPer1kOutputTokens,
		NominalP50LatencyMs:   m.NominalP50LatencyMs,
		NominalP95LatencyMs:   m.NominalP95LatencyMs,
		TaskQualityMatrix:     m.TaskQualityMatrix,
		HealthStatus:          string(m.HealthStatus),
		ObservedMetrics:       m.ObservedMetrics,
	}
}

func sortModels(models []ModelResponse) {
	tierOrder := map[string]int{"small": 0, "medium": 1, "large": 2}
	for i := 0; i < len(models)-1; i++ {
		for j := i + 1; j < len(models); j++ {
			if tierOrder[models[i].Tier] > tierOrder[models[j].Tier] {
				models[i], models[j] = models[j], models[i]
			}
		}
	}
	_ = strconv.Itoa // suppress unused import
}
