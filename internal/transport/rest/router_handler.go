package rest

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/sentinelmesh/sentinelmesh/internal/router"
)

// RouterHandler exposes Stage 17 deterministic routing via REST.
type RouterHandler struct {
	svc          *router.Service
	decisionRepo router.DecisionRepository
}

// NewRouterHandler creates a RouterHandler.
func NewRouterHandler(svc *router.Service, decisionRepo router.DecisionRepository) *RouterHandler {
	return &RouterHandler{svc: svc, decisionRepo: decisionRepo}
}

// RegisterRoutes registers router endpoints.
func (h *RouterHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/router/route", h.Route)
	mux.HandleFunc("POST /v1/router/execute", h.Execute)
	mux.HandleFunc("GET /v1/router/decisions", h.ListDecisions)
	mux.HandleFunc("GET /v1/router/outcomes", h.ListOutcomes)
}

// RouteRequest is the JSON body for POST /v1/router/route
type RouteRequest struct {
	Prompt                string  `json:"prompt"`
	TaskComplexity        string  `json:"task_complexity"`
	QualityRequirement    float64 `json:"quality_requirement"`
	LatencySLAMs          float64 `json:"latency_sla_ms"`
	CostBudgetUSD         float64 `json:"cost_budget_usd"`
	EstimatedInputTokens  int     `json:"estimated_input_tokens"`
	EstimatedOutputTokens int     `json:"estimated_output_tokens"`
	SecurityProfile       string  `json:"security_profile"`
	RoutingPolicy         string  `json:"routing_policy"`
	PinnedModelID         string  `json:"pinned_model_id,omitempty"`
	TaskID                string  `json:"task_id,omitempty"`
	AgentID               string  `json:"agent_id,omitempty"`
	TenantID              string  `json:"tenant_id,omitempty"`
}

// Route handles POST /v1/router/route — Stage 17 deterministic routing.
func (h *RouterHandler) Route(w http.ResponseWriter, r *http.Request) {
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
		// Even a failure may contain partial decision data (rejections).
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

// Execute handles POST /v1/router/execute — Stage 17 + model invocation.
func (h *RouterHandler) Execute(w http.ResponseWriter, r *http.Request) {
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

	resp, err := h.svc.Execute(r.Context(), routingReq)
	if err != nil {
		WriteError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// ListDecisions handles GET /v1/router/decisions
func (h *RouterHandler) ListDecisions(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	limit := 50
	if limitStr != "" {
		if v, err := strconv.Atoi(limitStr); err == nil && v > 0 {
			limit = v
		}
	}

	decisions, err := h.decisionRepo.ListDecisions(r.Context(), limit)
	if err != nil {
		WriteError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"decisions": decisions,
		"count":     len(decisions),
	})
}

// ListOutcomes handles GET /v1/router/outcomes
func (h *RouterHandler) ListOutcomes(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	limit := 50
	if limitStr != "" {
		if v, err := strconv.Atoi(limitStr); err == nil && v > 0 {
			limit = v
		}
	}

	outcomes, err := h.decisionRepo.ListOutcomes(r.Context(), limit)
	if err != nil {
		WriteError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"outcomes": outcomes,
		"count":    len(outcomes),
	})
}
