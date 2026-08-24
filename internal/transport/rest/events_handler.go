package rest

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/sentinelmesh/sentinelmesh/internal/events"
	"github.com/sentinelmesh/sentinelmesh/internal/outbox"
)

// EventsHandler handles event stream and event list endpoints.
type EventsHandler struct {
	outboxRepo outbox.Repository
	hub        *SSEHub
}

// NewEventsHandler creates an EventsHandler.
func NewEventsHandler(outboxRepo outbox.Repository, hub *SSEHub) *EventsHandler {
	return &EventsHandler{outboxRepo: outboxRepo, hub: hub}
}

// RegisterRoutes registers event endpoints.
func (h *EventsHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/events", h.ListEvents)
	mux.Handle("GET /v1/events/stream", h.hub)
}

// EventListResponse is a paginated event list.
type EventListResponse struct {
	Events []events.Event `json:"events"`
	Count  int            `json:"count"`
}

// ListEvents handles GET /v1/events — paginated recent events with optional filtering.
func (h *EventsHandler) ListEvents(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	limitStr := q.Get("limit")
	limit := 100
	if limitStr != "" {
		if v, err := strconv.Atoi(limitStr); err == nil && v > 0 && v <= 1000 {
			limit = v
		}
	}

	eventTypeFilter := q.Get("type")
	stageFilter := q.Get("stage")
	taskIDFilter := q.Get("task_id")

	// Only MemoryRepository exposes GetEvents() directly.
	// For DB-backed repos, we'd query; for now use the in-process store.
	var all []events.Event
	if memRepo, ok := h.outboxRepo.(*outbox.MemoryRepository); ok {
		all = memRepo.GetEvents()
	}

	// Filter and paginate.
	filtered := filterEvents(all, eventTypeFilter, stageFilter, taskIDFilter)

	// Most-recent first.
	for i, j := 0, len(filtered)-1; i < j; i, j = i+1, j-1 {
		filtered[i], filtered[j] = filtered[j], filtered[i]
	}

	if limit < len(filtered) {
		filtered = filtered[:limit]
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(EventListResponse{
		Events: filtered,
		Count:  len(filtered),
	})
}

func filterEvents(evts []events.Event, typeFilter, stageFilter, taskIDFilter string) []events.Event {
	if typeFilter == "" && stageFilter == "" && taskIDFilter == "" {
		return evts
	}

	stageSubjects := map[string][]string{
		"stage17": {"sentinel.router.v1"},
		"stage18": {"sentinel.adaptive.v1"},
		"stage19": {"sentinel.policy.v2"},
		"agent":   {"sentinel.agent.v1"},
		"run":     {"sentinel.run.v1"},
	}

	var out []events.Event
	for _, e := range evts {
		if typeFilter != "" && !strings.EqualFold(e.EventType, typeFilter) {
			continue
		}
		if stageFilter != "" {
			subjects, ok := stageSubjects[strings.ToLower(stageFilter)]
			if ok {
				matched := false
				for _, s := range subjects {
					if strings.HasPrefix(e.EventType, s) {
						matched = true
						break
					}
				}
				if !matched {
					continue
				}
			}
		}
		if taskIDFilter != "" && e.AggregateID != taskIDFilter {
			continue
		}
		out = append(out, e)
	}
	return out
}
