package rest

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sentinelmesh/sentinelmesh/internal/events"
	"github.com/sentinelmesh/sentinelmesh/internal/outbox"
)

// OutboxSSEBridge polls the outbox memory repository and forwards new events
// to the SSEHub, enabling real-time browser event streaming without NATS.
type OutboxSSEBridge struct {
	repo       *outbox.MemoryRepository
	hub        *SSEHub
	pollInterval time.Duration
	lastIdx    atomic.Int64
	stopCh     chan struct{}
	wg         sync.WaitGroup
}

// NewOutboxSSEBridge creates a bridge between the in-process outbox and the SSE hub.
func NewOutboxSSEBridge(repo *outbox.MemoryRepository, hub *SSEHub, pollInterval time.Duration) *OutboxSSEBridge {
	if pollInterval <= 0 {
		pollInterval = 200 * time.Millisecond
	}
	return &OutboxSSEBridge{
		repo:         repo,
		hub:          hub,
		pollInterval: pollInterval,
		stopCh:       make(chan struct{}),
	}
}

// Start begins the polling loop in a background goroutine.
func (b *OutboxSSEBridge) Start(ctx context.Context) {
	b.wg.Add(1)
	go func() {
		defer b.wg.Done()
		ticker := time.NewTicker(b.pollInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-b.stopCh:
				return
			case <-ticker.C:
				b.poll()
			}
		}
	}()
	slog.Info("OutboxSSEBridge started", "poll_interval", b.pollInterval)
}

// Stop shuts down the polling goroutine cleanly.
func (b *OutboxSSEBridge) Stop() {
	close(b.stopCh)
	b.wg.Wait()
	slog.Info("OutboxSSEBridge stopped")
}

func (b *OutboxSSEBridge) poll() {
	all := b.repo.GetEvents()
	lastIdx := int(b.lastIdx.Load())
	if len(all) <= lastIdx {
		return
	}

	newEvts := all[lastIdx:]
	b.lastIdx.Store(int64(len(all)))

	for _, e := range newEvts {
		sseEvt := domainEventToSSE(e)
		b.hub.Publish(sseEvt)
	}
}

// domainEventToSSE converts a domain event to an SSE envelope.
func domainEventToSSE(e events.Event) SSEEvent {
	stage := classifyStage(e.EventType)

	return SSEEvent{
		ID:        e.EventID,
		Type:      eventTypeLabel(e.EventType),
		Timestamp: e.OccurredAt,
		Stage:     stage,
		TaskID:    e.AggregateID,
		TenantID:  e.TenantID,
		Status:    "ok",
		Payload:   e.Payload,
	}
}

// classifyStage maps NATS subject prefixes to UI stage identifiers.
func classifyStage(eventType string) string {
	switch {
	case len(eventType) >= 18 && eventType[:18] == "sentinel.router.v1":
		return "stage17"
	case len(eventType) >= 20 && eventType[:20] == "sentinel.adaptive.v1":
		return "stage18"
	case len(eventType) >= 19 && eventType[:19] == "sentinel.policy.v2.":
		return "stage19"
	case len(eventType) >= 17 && eventType[:17] == "sentinel.agent.v1":
		return "agent"
	case len(eventType) >= 15 && eventType[:15] == "sentinel.run.v1":
		return "run"
	case len(eventType) >= 22 && eventType[:22] == "sentinel.checkpoint.v1":
		return "checkpoint"
	case len(eventType) >= 20 && eventType[:20] == "sentinel.security.v1":
		return "security"
	case len(eventType) >= 19 && eventType[:19] == "sentinel.cluster.v1":
		return "cluster"
	default:
		return "system"
	}
}

// eventTypeLabel converts a full NATS subject into a short readable label.
func eventTypeLabel(subject string) string {
	labels := map[string]string{
		"sentinel.router.v1.decided":                 "ROUTING_DECIDED",
		"sentinel.router.v1.invocation_completed":    "INVOCATION_COMPLETED",
		"sentinel.router.v1.invocation_failed":       "INVOCATION_FAILED",
		"sentinel.router.v1.fallback_triggered":      "FALLBACK_TRIGGERED",
		"sentinel.adaptive.v1.decided":               "ADAPTIVE_DECIDED",
		"sentinel.adaptive.v1.drift_detected":        "DRIFT_DETECTED",
		"sentinel.policy.v2.decided":                 "POLICY_DECIDED",
		"sentinel.policy.v2.rollback_triggered":      "POLICY_ROLLBACK",
		"sentinel.policy.v2.shadow_evaluated":        "SHADOW_EVALUATED",
		"sentinel.agent.v1.created":                  "AGENT_CREATED",
		"sentinel.agent.v1.deleted":                  "AGENT_DELETED",
		"sentinel.run.v1.created":                    "RUN_CREATED",
		"sentinel.run.v1.state_changed":              "RUN_STATE_CHANGED",
		"sentinel.run.v1.scheduled":                  "RUN_SCHEDULED",
		"sentinel.run.v1.scheduling_failed":          "RUN_SCHEDULING_FAILED",
		"sentinel.run.v1.execution_fenced":           "EXECUTION_FENCED",
		"sentinel.run.v1.recovery_requested":         "RECOVERY_REQUESTED",
		"sentinel.run.v1.recovered":                  "RUN_RECOVERED",
		"sentinel.run.v1.recovery_failed":            "RECOVERY_FAILED",
		"sentinel.run.v1.verification_requested":     "VERIFICATION_REQUESTED",
		"sentinel.run.v1.verified":                   "RUN_VERIFIED",
		"sentinel.run.v1.verification_failed":        "VERIFICATION_FAILED",
		"sentinel.checkpoint.v1.saved":               "CHECKPOINT_SAVED",
		"sentinel.cluster.v1.node_failed":            "CLUSTER_NODE_FAILED",
		"sentinel.cluster.v1.unreachable":            "CLUSTER_UNREACHABLE",
		"sentinel.cluster.v1.heartbeat":              "CLUSTER_HEARTBEAT",
		"sentinel.security.v1.policy_violation":      "SECURITY_VIOLATION",
		"sentinel.security.v1.sandbox_violation":     "SANDBOX_VIOLATION",
	}

	if label, ok := labels[subject]; ok {
		return label
	}
	return subject
}

// PublishSystemEvent publishes a synthetic system-level event (e.g., connection, health).
func PublishSystemEvent(hub *SSEHub, eventType, status, detail string) {
	payload, _ := json.Marshal(map[string]string{"detail": detail})
	hub.Publish(SSEEvent{
		ID:        fmt.Sprintf("sys-%d", time.Now().UnixNano()),
		Type:      eventType,
		Timestamp: time.Now().UTC(),
		Stage:     "system",
		Status:    status,
		Payload:   payload,
	})
}
