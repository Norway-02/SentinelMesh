package operator

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"go.opentelemetry.io/otel/attribute"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/sentinelmesh/sentinelmesh/internal/events"
	"github.com/sentinelmesh/sentinelmesh/internal/messaging"
	"github.com/sentinelmesh/sentinelmesh/internal/observability"
)

// Namespace is the dedicated SentinelMesh namespace.
const Namespace = "sentinelmesh"

// EventConsumer subscribes to RunScheduled events from NATS JetStream and
// translates them into AgentRun CRs in Kubernetes.
type EventConsumer struct {
	natsClient *messaging.NATSClient
	k8sClient  client.Client
	clusterID  string
}

// NewEventConsumer constructs an EventConsumer.
func NewEventConsumer(nc *messaging.NATSClient, k8sClient client.Client) *EventConsumer {
	return &EventConsumer{
		natsClient: nc,
		k8sClient:  k8sClient,
	}
}

// WithClusterID configures cluster-specific targeted subject routing for the operator.
func (c *EventConsumer) WithClusterID(clusterID string) *EventConsumer {
	c.clusterID = clusterID
	return c
}

// Start begins consuming RunScheduled events from the NATS JetStream.
func (c *EventConsumer) Start(ctx context.Context) error {
	js := c.natsClient.JetStream()

	durableName := "sentinel-operator"
	filterSubject := events.SubjectRunScheduled

	if c.clusterID != "" {
		durableName = "sentinel-operator-" + c.clusterID
		filterSubject = events.SubjectRunScheduledForCluster(c.clusterID)
	}

	cons, err := js.CreateOrUpdateConsumer(ctx, "SENTINEL_RUN", jetstream.ConsumerConfig{
		Durable:       durableName,
		AckPolicy:     jetstream.AckExplicitPolicy,
		FilterSubject: filterSubject,
		MaxDeliver:    5,
	})
	if err != nil {
		return fmt.Errorf("failed to create NATS consumer: %w", err)
	}

	iter, err := cons.Messages()
	if err != nil {
		return fmt.Errorf("failed to get NATS message iterator: %w", err)
	}

	slog.Info("Operator consumer started, waiting for RunScheduled events...",
		"cluster_id", c.clusterID,
		"filter_subject", filterSubject,
		"namespace", Namespace)

	go func() {
		for {
			select {
			case <-ctx.Done():
				slog.Info("Shutting down event consumer")
				iter.Stop()
				return
			default:
				msg, err := iter.Next()
				if err != nil {
					time.Sleep(1 * time.Second)
					continue
				}
				c.handleMessage(ctx, msg)
			}
		}
	}()

	return nil
}

// handleMessage processes a single RunScheduled NATS message.
func (c *EventConsumer) handleMessage(ctx context.Context, msg jetstream.Msg) {
	// Unwrap the Event envelope
	var envelope events.Event
	if err := json.Unmarshal(msg.Data(), &envelope); err != nil {
		slog.Error("Failed to decode event envelope", "error", err)
		msg.Ack() // Poison message — don't retry
		return
	}

	// Decode the RunScheduled payload
	var payload events.RunScheduledPayload
	if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
		slog.Error("Failed to decode RunScheduledPayload", "error", err)
		msg.Ack() // Poison message — don't retry
		return
	}

	// Cluster-Targeted Filter Check
	if c.clusterID != "" && payload.ClusterID != "" && payload.ClusterID != c.clusterID {
		slog.Warn("Ignoring event targeted for different cluster",
			"event_cluster", payload.ClusterID,
			"operator_cluster", c.clusterID,
			"run_id", payload.RunID,
		)
		msg.Ack()
		return
	}

	var msgHeaders nats.Header
	if msg.Headers() != nil {
		msgHeaders = nats.Header(msg.Headers())
	}
	msgCtx := observability.ExtractNATSHeaders(ctx, msgHeaders)
	if envelope.TraceParent != "" && (msgHeaders == nil || msgHeaders.Get("traceparent") == "") {
		hdr := make(http.Header)
		hdr.Set("traceparent", envelope.TraceParent)
		msgCtx = observability.ExtractHTTPHeaders(msgCtx, hdr)
	}

	msgCtx = observability.WithRunID(msgCtx, payload.RunID)
	msgCtx = observability.WithAgentID(msgCtx, payload.AgentID)
	msgCtx = observability.WithTenantID(msgCtx, envelope.TenantID)
	msgCtx = observability.WithCorrelationID(msgCtx, envelope.CorrelationID)
	if payload.ExecutionGeneration > 0 {
		msgCtx = observability.WithExecutionGeneration(msgCtx, payload.ExecutionGeneration)
	}

	msgCtx, span := observability.StartConsumerSpan(msgCtx, "operator.consume_run_scheduled")
	defer span.End()

	observability.InjectSpanAttributes(span, payload.RunID, payload.AgentID, envelope.TenantID, envelope.CorrelationID, payload.ExecutionGeneration)
	span.SetAttributes(
		attribute.String("operator.node_id", payload.NodeID),
		attribute.String("operator.cluster_id", payload.ClusterID),
		attribute.String("operator.fencing_token", payload.FencingToken),
		attribute.Int("operator.execution_generation", payload.ExecutionGeneration),
	)

	// Validate before touching Kubernetes
	if err := validateRunScheduledPayload(&payload); err != nil {
		observability.RecordSpanError(span, err)
		slog.ErrorContext(msgCtx, "RunScheduled event failed validation",
			"run_id", payload.RunID,
			"reason", err.Error(),
		)
		msg.Ack()
		return
	}

	slog.InfoContext(msgCtx, "Received valid RunScheduled event",
		"run_id", payload.RunID,
		"agent_id", payload.AgentID,
		"cluster_id", payload.ClusterID,
		"node_id", payload.NodeID,
		"execution_generation", payload.ExecutionGeneration,
		"fencing_token", payload.FencingToken,
		"image", payload.AgentImage,
	)

	// Translate event → desired CR state
	agentRun := MapRunScheduledToAgentRun(&payload, Namespace)

	err := c.k8sClient.Create(msgCtx, agentRun)
	if err != nil {
		if errors.IsAlreadyExists(err) {
			slog.InfoContext(msgCtx, "AgentRun already exists (idempotent), skipping",
				"run_id", payload.RunID,
				"name", agentRun.Name,
			)
			msg.Ack()
			return
		}
		observability.RecordSpanError(span, err)
		slog.ErrorContext(msgCtx, "Failed to create AgentRun in Kubernetes",
			"error", err,
			"run_id", payload.RunID,
		)
		msg.Nak()
		return
	}

	slog.InfoContext(msgCtx, "AgentRun created in Kubernetes",
		"run_id", payload.RunID,
		"name", agentRun.Name,
		"namespace", agentRun.Namespace,
		"cluster", payload.ClusterID,
		"node", payload.NodeID,
		"execution_generation", payload.ExecutionGeneration,
	)
	msg.Ack()
}

func validateRunScheduledPayload(p *events.RunScheduledPayload) error {
	if p.RunID == "" {
		return fmt.Errorf("run_id is required")
	}
	if p.AgentID == "" {
		return fmt.Errorf("agent_id is required")
	}
	if p.NodeID == "" {
		return fmt.Errorf("node_id is required")
	}
	if p.AgentImage == "" {
		return fmt.Errorf("agent_image is required")
	}
	if p.AgentCPU == "" {
		return fmt.Errorf("agent_cpu is required")
	}
	if p.AgentMemory == "" {
		return fmt.Errorf("agent_memory is required")
	}

	if _, err := resource.ParseQuantity(p.AgentCPU); err != nil {
		return fmt.Errorf("agent_cpu %q is not a valid resource quantity: %w", p.AgentCPU, err)
	}
	if _, err := resource.ParseQuantity(p.AgentMemory); err != nil {
		return fmt.Errorf("agent_memory %q is not a valid resource quantity: %w", p.AgentMemory, err)
	}

	return nil
}
