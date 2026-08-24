package rest

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// SSEEvent is the payload sent to browser clients.
type SSEEvent struct {
	ID        string          `json:"id"`
	Type      string          `json:"type"`
	Timestamp time.Time       `json:"timestamp"`
	Stage     string          `json:"stage,omitempty"`
	TaskID    string          `json:"task_id,omitempty"`
	RunID     string          `json:"run_id,omitempty"`
	AgentID   string          `json:"agent_id,omitempty"`
	TenantID  string          `json:"tenant_id,omitempty"`
	Status    string          `json:"status,omitempty"`
	Payload   json.RawMessage `json:"payload,omitempty"`
}

// sseClient represents a connected browser client.
type sseClient struct {
	id      uint64
	ch      chan SSEEvent
	lastID  string
	closeCh chan struct{}
}

// SSEHub manages Server-Sent Events fan-out to browser clients.
// One slow client never blocks others (bounded channel, drop on full).
type SSEHub struct {
	mu         sync.RWMutex
	clients    map[uint64]*sseClient
	nextID     atomic.Uint64
	bufferSize int
}

// NewSSEHub creates a new SSEHub.
func NewSSEHub() *SSEHub {
	return &SSEHub{
		clients:    make(map[uint64]*sseClient),
		bufferSize: 256,
	}
}

// Subscribe registers a new client and returns its channel and unsubscribe func.
func (h *SSEHub) Subscribe() (*sseClient, func()) {
	id := h.nextID.Add(1)
	c := &sseClient{
		id:      id,
		ch:      make(chan SSEEvent, h.bufferSize),
		closeCh: make(chan struct{}),
	}

	h.mu.Lock()
	h.clients[id] = c
	h.mu.Unlock()

	unsub := func() {
		h.mu.Lock()
		delete(h.clients, id)
		h.mu.Unlock()
		close(c.closeCh)
	}

	return c, unsub
}

// Publish broadcasts an event to all connected clients.
// Slow clients have their event dropped (never blocked).
func (h *SSEHub) Publish(evt SSEEvent) {
	h.mu.RLock()
	clients := make([]*sseClient, 0, len(h.clients))
	for _, c := range h.clients {
		clients = append(clients, c)
	}
	h.mu.RUnlock()

	for _, c := range clients {
		select {
		case c.ch <- evt:
		default:
			// Client buffer full — drop event to preserve non-blocking guarantee.
			slog.Warn("SSE client buffer full, dropping event", "client_id", c.id, "event_type", evt.Type)
		}
	}
}

// ClientCount returns the number of currently connected SSE clients.
func (h *SSEHub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// ServeHTTP handles the SSE stream endpoint.
// GET /v1/events/stream
func (h *SSEHub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// SSE requires flushing.
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("X-Accel-Buffering", "no") // nginx passthrough

	client, unsub := h.Subscribe()
	defer unsub()

	// Heartbeat ticker so browser knows connection is alive.
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	// Send initial connection confirmation.
	connectEvt := SSEEvent{
		ID:        fmt.Sprintf("hub-%d", client.id),
		Type:      "CONNECTED",
		Timestamp: time.Now().UTC(),
		Status:    "ok",
	}
	writeSSEEvent(w, connectEvt)
	flusher.Flush()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-client.closeCh:
			return
		case <-ticker.C:
			// SSE heartbeat comment.
			fmt.Fprintf(w, ": heartbeat\n\n")
			flusher.Flush()
		case evt, ok := <-client.ch:
			if !ok {
				return
			}
			writeSSEEvent(w, evt)
			flusher.Flush()
		}
	}
}

// writeSSEEvent serializes one SSE event to the response writer.
func writeSSEEvent(w http.ResponseWriter, evt SSEEvent) {
	if evt.ID != "" {
		fmt.Fprintf(w, "id: %s\n", evt.ID)
	}
	if evt.Type != "" {
		fmt.Fprintf(w, "event: %s\n", evt.Type)
	}
	data, _ := json.Marshal(evt)
	fmt.Fprintf(w, "data: %s\n\n", data)
}
