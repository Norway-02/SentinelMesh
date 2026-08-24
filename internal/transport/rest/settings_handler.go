package rest

import (
	"encoding/json"
	"net/http"

	"github.com/sentinelmesh/sentinelmesh/internal/config"
	"github.com/sentinelmesh/sentinelmesh/internal/onlinepolicy"
	"github.com/sentinelmesh/sentinelmesh/internal/router"
)

// SettingsHandler exposes safe non-secret configuration information.
type SettingsHandler struct {
	cfg        *config.Config
	registry   router.Registry
	policyMgr  *onlinepolicy.PolicyManager
	provider   router.ModelProvider
}

// NewSettingsHandler creates a SettingsHandler.
func NewSettingsHandler(
	cfg *config.Config,
	registry router.Registry,
	policyMgr *onlinepolicy.PolicyManager,
	provider router.ModelProvider,
) *SettingsHandler {
	return &SettingsHandler{
		cfg:       cfg,
		registry:  registry,
		policyMgr: policyMgr,
		provider:  provider,
	}
}

// RegisterRoutes registers settings endpoints.
func (h *SettingsHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/settings", h.GetSettings)
}

// GetSettings handles GET /v1/settings — returns safe non-secret configuration.
// NEVER returns: API keys, tokens, private keys, passwords, database credentials.
func (h *SettingsHandler) GetSettings(w http.ResponseWriter, r *http.Request) {
	models, _ := h.registry.ListModels(r.Context())

	policyState := h.policyMgr.GetActiveState()

	// Detect execution mode from provider if it's a LiveModelProvider.
	executionMode := "SYNTHETIC"
	if lp, ok := h.provider.(*router.LiveModelProvider); ok {
		executionMode = string(lp.GetMode())
	}

	resp := map[string]interface{}{
		// Never expose NATS URL, Database URL, API keys.
		"environment":         h.cfg.Environment,
		"http_addr":           h.cfg.HTTPAddr,
		"log_level":           h.cfg.LogLevel,
		"execution_mode":      executionMode,
		"provider_count":      len(models),
		"registry_version":    h.registry.Version(),
		"router_version":      router.AlgorithmVersion,
		"policy_version":      policyState.Version,
		"policy_parent":       policyState.ParentVersion,
		"policy_mode":         string(policyState.Mode),
		"adaptive_version":    "adaptive-v1.0",
		"drift_detector":      "drift-v1.0",
		"nats_configured":     h.cfg.NATSURL != "",
		"database_configured": h.cfg.DatabaseURL != "",
		"openai_configured":   h.cfg.OpenAIAPIKey != "",
		"openai_model":        h.cfg.OpenAIMediumModel,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
