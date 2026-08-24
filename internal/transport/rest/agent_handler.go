package rest

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/sentinelmesh/sentinelmesh/internal/application"
	"github.com/sentinelmesh/sentinelmesh/internal/domain"
	"github.com/sentinelmesh/sentinelmesh/internal/repository"
	"github.com/sentinelmesh/sentinelmesh/internal/types"
)

type AgentHandler struct {
	svc *application.AgentService
}

func NewAgentHandler(svc *application.AgentService) *AgentHandler {
	return &AgentHandler{svc: svc}
}

func (h *AgentHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/agents", h.CreateAgent)
	mux.HandleFunc("GET /v1/agents", h.ListAgents)
	mux.HandleFunc("GET /v1/agents/{id}", h.GetAgent)
	mux.HandleFunc("DELETE /v1/agents/{id}", h.DeleteAgent)
}

type CreateAgentRequest struct {
	Name           string               `json:"name"`
	Version        string               `json:"version"`
	TenantID       string               `json:"tenant_id"`
	Image          string               `json:"image"`
	Priority       string               `json:"priority"`
	Resources      types.AgentResources `json:"resources"`
	SecurityPolicy types.SecurityPolicy `json:"security_policy"`
}

func (h *AgentHandler) CreateAgent(w http.ResponseWriter, r *http.Request) {
	var req CreateAgentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, domain.ErrInvalidAgent)
		return
	}

	input := domain.Agent{
		Name:           req.Name,
		Version:        req.Version,
		TenantID:       req.TenantID,
		Image:          req.Image,
		Priority:       req.Priority,
		Resources:      req.Resources,
		SecurityPolicy: req.SecurityPolicy,
	}

	agent, err := h.svc.CreateAgent(r.Context(), input)
	if err != nil {
		WriteError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(agent)
}

func (h *AgentHandler) GetAgent(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		WriteError(w, repository.ErrNotFound)
		return
	}

	agent, err := h.svc.GetAgent(r.Context(), id)
	if err != nil {
		WriteError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(agent)
}

func (h *AgentHandler) ListAgents(w http.ResponseWriter, r *http.Request) {
	tenantID := r.URL.Query().Get("tenant_id")
	pageSizeStr := r.URL.Query().Get("page_size")
	pageToken := r.URL.Query().Get("page_token")

	pageSize := 50
	if pageSizeStr != "" {
		if val, err := strconv.Atoi(pageSizeStr); err == nil {
			pageSize = val
		}
	}

	filter := repository.AgentFilter{
		TenantID:  tenantID,
		PageSize:  pageSize,
		PageToken: pageToken,
	}

	agents, nextToken, err := h.svc.ListAgents(r.Context(), filter)
	if err != nil {
		WriteError(w, err)
		return
	}

	// Prepare response
	resp := struct {
		Agents        []domain.Agent `json:"agents"`
		NextPageToken string         `json:"next_page_token"`
	}{
		Agents:        agents,
		NextPageToken: nextToken,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *AgentHandler) DeleteAgent(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		WriteError(w, repository.ErrNotFound)
		return
	}

	if err := h.svc.DeleteAgent(r.Context(), id); err != nil {
		WriteError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
