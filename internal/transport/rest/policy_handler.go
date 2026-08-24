package rest

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/sentinelmesh/sentinelmesh/internal/adaptive"
	"github.com/sentinelmesh/sentinelmesh/internal/onlinepolicy"
	"github.com/sentinelmesh/sentinelmesh/internal/router"
)

// PolicyHandler exposes Stage 19 online policy learning via REST.
type PolicyHandler struct {
	svc          *onlinepolicy.OnlinePolicyService
	adaptiveSvc  *adaptive.AdaptiveService
	routerSvc    *router.Service
	policyMgr    *onlinepolicy.PolicyManager
}

// NewPolicyHandler creates a PolicyHandler.
func NewPolicyHandler(
	svc *onlinepolicy.OnlinePolicyService,
	adaptiveSvc *adaptive.AdaptiveService,
	routerSvc *router.Service,
	policyMgr *onlinepolicy.PolicyManager,
) *PolicyHandler {
	return &PolicyHandler{
		svc:         svc,
		adaptiveSvc: adaptiveSvc,
		routerSvc:   routerSvc,
		policyMgr:   policyMgr,
	}
}

// RegisterRoutes registers policy endpoints.
func (h *PolicyHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/policy/route", h.Route)
	mux.HandleFunc("POST /v1/policy/execute", h.Execute)
	mux.HandleFunc("GET /v1/policy/state", h.GetState)
}

// Route handles POST /v1/policy/route — Stage 19 policy decision only.
func (h *PolicyHandler) Route(w http.ResponseWriter, r *http.Request) {
	var req RouteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	taskID := req.TaskID
	if taskID == "" {
		taskID = "task-" + uuid.New().String()
	}

	routingReq := buildRoutingRequest(req, taskID)

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

// ExecuteResponse is the complete pipeline result returned to the UI.
// It bundles the Stage17, Stage18, Stage19 decisions with the actual invocation result.
type ExecuteResponse struct {
	TaskID           string                          `json:"task_id"`
	RunID            string                          `json:"run_id"`
	ExecutedAt       time.Time                       `json:"executed_at"`
	Stage17Decision  *router.RoutingDecision          `json:"stage17_decision,omitempty"`
	Stage18Decision  *adaptive.AdaptiveRoutingDecision `json:"stage18_decision,omitempty"`
	Stage19Decision  *onlinepolicy.PolicyDecision     `json:"stage19_decision,omitempty"`
	InvocationResult *router.ModelInvocationResponse  `json:"invocation_result,omitempty"`
	ExecutionMode    string                           `json:"execution_mode"`
	Error            string                           `json:"error,omitempty"`
}

// Execute handles POST /v1/policy/execute — complete Stage 17 → 18 → 19 → invocation pipeline.
// The browser calls ONE endpoint. The backend orchestrates all stages.
func (h *PolicyHandler) Execute(w http.ResponseWriter, r *http.Request) {
	var req RouteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	taskID := req.TaskID
	if taskID == "" {
		taskID = "task-" + uuid.New().String()
	}
	runID := "run-" + uuid.New().String()

	routingReq := buildRoutingRequest(req, taskID)
	routingReq.RunID = runID

	result := ExecuteResponse{
		TaskID:        taskID,
		RunID:         runID,
		ExecutedAt:    time.Now().UTC(),
		ExecutionMode: "SYNTHETIC",
	}

	// Stage 17 — deterministic routing (safe feasibility set).
	stage17, err := h.routerSvc.Route(r.Context(), routingReq)
	if err != nil {
		result.Error = "stage17: " + err.Error()
		result.Stage17Decision = &stage17
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		json.NewEncoder(w).Encode(result)
		return
	}
	result.Stage17Decision = &stage17

	// Stage 18 — adaptive Bayesian prediction.
	stage18, err := h.adaptiveSvc.Route(r.Context(), routingReq)
	if err != nil {
		result.Error = "stage18: " + err.Error()
		result.Stage18Decision = &stage18
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		json.NewEncoder(w).Encode(result)
		return
	}
	result.Stage18Decision = &stage18

	// Stage 19 — online policy execute (includes invocation + reward + guardrails).
	invResp, err := h.svc.Execute(r.Context(), routingReq)
	if err != nil {
		result.Error = "stage19: " + err.Error()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		json.NewEncoder(w).Encode(result)
		return
	}
	result.InvocationResult = invResp
	if invResp.DataSource != "" {
		result.ExecutionMode = invResp.DataSource
	} else {
		result.ExecutionMode = "LIVE"
	}

	// Retrieve the policy decision for the response (re-route to get the decision object).
	policyDec, _ := h.svc.Route(r.Context(), routingReq)
	result.Stage19Decision = &policyDec

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// GetState handles GET /v1/policy/state — returns active policy state (safe subset).
func (h *PolicyHandler) GetState(w http.ResponseWriter, r *http.Request) {
	state := h.policyMgr.GetActiveState()

	// Return safe non-secret policy metadata.
	resp := map[string]interface{}{
		"version":                   state.Version,
		"parent_version":            state.ParentVersion,
		"mode":                      string(state.Mode),
		"exploration_lambda":        state.ExplorationLambda,
		"exploration_budget":        state.ExplorationBudget,
		"global_exploration_limit":  state.GlobalExplorationLimit,
		"per_model_exploration_limit": state.PerModelExplorationLimit,
		"window_size":               state.WindowSize,
		"total_decisions":           state.TotalDecisions,
		"exploration_count":         state.ExplorationCount,
		"exploitation_count":        state.ExploitationCount,
		"reward_weights":            state.RewardWeights,
		"is_rolled_back":            state.IsRolledBack,
		"last_rollback":             state.LastRollback,
		"created_at":                state.CreatedAt,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// buildRoutingRequest constructs a RoutingRequest from the HTTP RouteRequest.
func buildRoutingRequest(req RouteRequest, taskID string) router.RoutingRequest {
	policy := req.RoutingPolicy
	if policy == "" {
		policy = string(router.PolicyBalanced)
	}
	complexity := req.TaskComplexity
	if complexity == "" {
		complexity = string(router.ComplexityModerate)
	}
	secProfile := req.SecurityProfile
	if secProfile == "" {
		secProfile = "standard"
	}
	inTokens := req.EstimatedInputTokens
	if inTokens == 0 {
		inTokens = len(req.Prompt) / 4
		if inTokens < 50 {
			inTokens = 50
		}
	}
	outTokens := req.EstimatedOutputTokens
	if outTokens == 0 {
		outTokens = 256
	}

	return router.RoutingRequest{
		TaskID:                taskID,
		RunID:                 "run-" + uuid.New().String(),
		AgentID:               req.AgentID,
		TenantID:              req.TenantID,
		Prompt:                req.Prompt,
		TaskComplexity:        router.TaskComplexity(complexity),
		QualityRequirement:    req.QualityRequirement,
		LatencySLAMs:          req.LatencySLAMs,
		CostBudgetUSD:         req.CostBudgetUSD,
		EstimatedInputTokens:  inTokens,
		EstimatedOutputTokens: outTokens,
		SecurityProfile:       secProfile,
		RoutingPolicy:         router.RoutingPolicy(policy),
		PinnedModelID:         req.PinnedModelID,
	}
}
