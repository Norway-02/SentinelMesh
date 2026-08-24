package rest

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/sentinelmesh/sentinelmesh/internal/adaptive"
	"github.com/sentinelmesh/sentinelmesh/internal/router"
)

// AdaptiveHandler exposes Stage 18 predictive adaptive routing via REST.
type AdaptiveHandler struct {
	svc *adaptive.AdaptiveService
}

// NewAdaptiveHandler creates an AdaptiveHandler.
func NewAdaptiveHandler(svc *adaptive.AdaptiveService) *AdaptiveHandler {
	return &AdaptiveHandler{svc: svc}
}

// RegisterRoutes registers adaptive routing endpoints.
func (h *AdaptiveHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/adaptive/route", h.Route)
}

// Route handles POST /v1/adaptive/route — Stage 18 adaptive prediction.
func (h *AdaptiveHandler) Route(w http.ResponseWriter, r *http.Request) {
	var req RouteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	taskID := req.TaskID
	if taskID == "" {
		taskID = "task-" + uuid.New().String()
	}

	routingReq := router.RoutingRequest{
		TaskID:                taskID,
		RunID:                 "run-" + uuid.New().String(),
		AgentID:               req.AgentID,
		TenantID:              req.TenantID,
		Prompt:                req.Prompt,
		TaskComplexity:        router.TaskComplexity(req.TaskComplexity),
		QualityRequirement:    req.QualityRequirement,
		LatencySLAMs:          req.LatencySLAMs,
		CostBudgetUSD:         req.CostBudgetUSD,
		EstimatedInputTokens:  req.EstimatedInputTokens,
		EstimatedOutputTokens: req.EstimatedOutputTokens,
		SecurityProfile:       req.SecurityProfile,
		RoutingPolicy:         router.RoutingPolicy(req.RoutingPolicy),
		PinnedModelID:         req.PinnedModelID,
	}

	decision, err := h.svc.Route(r.Context(), routingReq)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":    err.Error(),
			"decision": decision,
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(decision)
}
