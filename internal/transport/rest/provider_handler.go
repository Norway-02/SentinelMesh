package rest

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/sentinelmesh/sentinelmesh/internal/config"
	"github.com/sentinelmesh/sentinelmesh/internal/observability"
	"github.com/sentinelmesh/sentinelmesh/internal/router"
)

// ProviderHandler exposes safe provider health checks and gated test endpoints.
type ProviderHandler struct {
	cfg      *config.Config
	provider *router.LiveModelProvider
}

// NewProviderHandler constructs a ProviderHandler.
func NewProviderHandler(cfg *config.Config, provider *router.LiveModelProvider) *ProviderHandler {
	return &ProviderHandler{
		cfg:      cfg,
		provider: provider,
	}
}

// RegisterRoutes registers provider management endpoints.
func (h *ProviderHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/providers/openai/health", h.GetOpenAIHealth)
	mux.HandleFunc("POST /v1/providers/openai/test", h.TestOpenAI)
	mux.HandleFunc("POST /v1/providers/openai/test-failure", h.TestOpenAIFailure)
}

// GetOpenAIHealth handles GET /v1/providers/openai/health — returns safe non-secret provider state.
func (h *ProviderHandler) GetOpenAIHealth(w http.ResponseWriter, r *http.Request) {
	mode := h.provider.GetMode()
	isConfigured := h.cfg.OpenAIAPIKey != ""

	recStatus, lastErr := h.provider.GetProviderState("openai")

	status := recStatus
	if mode == router.ModeLive && !isConfigured {
		status = "NOT_CONFIGURED"
	}

	resp := map[string]interface{}{
		"provider":   "openai",
		"mode":       string(mode),
		"configured": isConfigured,
		"model":      h.cfg.OpenAIMediumModel,
		"status":     status,
		"last_error": lastErr,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

// TestOpenAI handles POST /v1/providers/openai/test — gated development test endpoint.
func (h *ProviderHandler) TestOpenAI(w http.ResponseWriter, r *http.Request) {
	if !h.cfg.EnableTestEndpoints {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Provider test endpoints are disabled in this environment",
		})
		return
	}

	var reqBody struct {
		Prompt string `json:"prompt"`
	}
	_ = json.NewDecoder(r.Body).Decode(&reqBody)
	if reqBody.Prompt == "" {
		reqBody.Prompt = "Reply with exactly: SENTINELMESH_LIVE_TEST"
	}

	ctx := r.Context()
	modelID := "medium-balanced-v1"

	invReq := router.ModelInvocationRequest{
		TaskID:         "test-task-001",
		RunID:          "test-run-001",
		AgentID:        "system-test",
		TenantID:       "default",
		Prompt:         reqBody.Prompt,
		TaskComplexity: router.ComplexitySimple,
		MaxTokens:      64,
		Timeout:        15 * time.Second,
	}

	invResp, err := h.provider.Invoke(ctx, modelID, invReq)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"provider": "openai",
			"error":    observability.RedactSecrets(err.Error()),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"provider":           "openai",
		"model":              invResp.ModelID,
		"output":             observability.RedactSecrets(invResp.Content),
		"latency_ms":         invResp.ActualLatency.Milliseconds(),
		"input_tokens":       invResp.PromptTokens,
		"output_tokens":      invResp.CompletionTokens,
		"estimated_cost_usd": invResp.EstimatedCostUSD,
		"actual_cost_usd":    invResp.ActualCostUSD,
		"success":            true,
	})
}

// TestOpenAIFailure handles POST /v1/providers/openai/test-failure — development fault injection toggle.
func (h *ProviderHandler) TestOpenAIFailure(w http.ResponseWriter, r *http.Request) {
	if !h.cfg.EnableTestEndpoints || h.cfg.Environment != "development" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Fault injection testing is only allowed in development mode",
		})
		return
	}

	var reqBody struct {
		EnableFaultInjection bool `json:"enable_fault_injection"`
	}
	_ = json.NewDecoder(r.Body).Decode(&reqBody)

	h.provider.SetFaultInjection(reqBody.EnableFaultInjection)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"provider":               "openai",
		"fault_injection_active": reqBody.EnableFaultInjection,
		"status":                 "updated",
	})
}
