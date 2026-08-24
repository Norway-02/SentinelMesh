package rest

import (
	"encoding/json"
	"net/http"

	"github.com/sentinelmesh/sentinelmesh/internal/application"
	"github.com/sentinelmesh/sentinelmesh/internal/repository"
)

type RunHandler struct {
	svc *application.RunService
}

func NewRunHandler(svc *application.RunService) *RunHandler {
	return &RunHandler{svc: svc}
}

func (h *RunHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/agents/{id}/runs", h.CreateRun)
	mux.HandleFunc("GET /v1/runs/{id}", h.GetRun)
	mux.HandleFunc("POST /v1/runs/{id}/cancel", h.CancelRun)
	mux.HandleFunc("GET /v1/runs/{id}/state", h.GetRunState)
}

func (h *RunHandler) CreateRun(w http.ResponseWriter, r *http.Request) {
	agentID := r.PathValue("id")
	if agentID == "" {
		WriteError(w, repository.ErrNotFound)
		return
	}

	run, err := h.svc.CreateRun(r.Context(), agentID)
	if err != nil {
		WriteError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(run)
}

func (h *RunHandler) GetRun(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		WriteError(w, repository.ErrNotFound)
		return
	}

	run, err := h.svc.GetRun(r.Context(), id)
	if err != nil {
		WriteError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(run)
}

func (h *RunHandler) CancelRun(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		WriteError(w, repository.ErrNotFound)
		return
	}

	run, err := h.svc.CancelRun(r.Context(), id)
	if err != nil {
		WriteError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(run)
}

func (h *RunHandler) GetRunState(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		WriteError(w, repository.ErrNotFound)
		return
	}

	state, err := h.svc.GetRunState(r.Context(), id)
	if err != nil {
		WriteError(w, err)
		return
	}

	resp := struct {
		State string `json:"state"`
	}{
		State: string(state),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
