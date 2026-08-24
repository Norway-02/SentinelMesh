package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"go.opentelemetry.io/otel/attribute"

	"github.com/sentinelmesh/sentinelmesh/internal/events"
	"github.com/sentinelmesh/sentinelmesh/internal/observability"
)

type NATSClient struct {
	nc *nats.Conn
	js jetstream.JetStream
}

func ConnectNATS(url string) (*NATSClient, error) {
	if url == "" {
		return nil, fmt.Errorf("nats url is required")
	}

	nc, err := nats.Connect(url)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to nats: %w", err)
	}

	js, err := jetstream.New(nc)
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("failed to create jetstream context: %w", err)
	}

	client := &NATSClient{
		nc: nc,
		js: js,
	}

	if err := client.ensureStreams(context.Background()); err != nil {
		return nil, err
	}

	return client, nil
}

func (c *NATSClient) ensureStreams(ctx context.Context) error {
	// Create Agent stream
	_, err := c.js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:     "SENTINEL_AGENT",
		Subjects: []string{"sentinel.agent.>"},
		Storage:  jetstream.FileStorage,
	})
	if err != nil {
		return fmt.Errorf("failed to configure agent stream: %w", err)
	}

	// Create Run stream
	_, err = c.js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:     "SENTINEL_RUN",
		Subjects: []string{"sentinel.run.>"},
		Storage:  jetstream.FileStorage,
	})
	if err != nil {
		return fmt.Errorf("failed to configure run stream: %w", err)
	}

	return nil
}

func (c *NATSClient) Publish(ctx context.Context, subject string, event events.Event) error {
	ctx, span := observability.StartProducerSpan(ctx, "nats.publish")
	defer span.End()

	m := observability.GetMetrics()

	observability.InjectSpanAttributes(span, event.AggregateID, "", event.TenantID, event.CorrelationID, 0)
	span.SetAttributes(
		attribute.String("messaging.system", "nats"),
		attribute.String("messaging.destination", subject),
		attribute.String("messaging.event_type", event.EventType),
		attribute.String("messaging.event_id", event.EventID),
	)

	msg := &nats.Msg{
		Subject: subject,
		Header:  nats.Header{},
	}

	// Inject W3C TraceContext into NATS message headers
	observability.InjectNATSHeaders(ctx, msg.Header)
	if event.TraceParent == "" {
		event.TraceParent = msg.Header.Get("traceparent")
	} else {
		msg.Header.Set("traceparent", event.TraceParent)
	}

	payload, err := json.Marshal(event)
	if err != nil {
		observability.RecordSpanError(span, err)
		m.OutboxPublishFailuresTotal.WithLabelValues(event.EventType).Inc()
		return fmt.Errorf("failed to marshal event for NATS: %w", err)
	}
	msg.Data = payload

	// Use event_id as JetStream message ID for strict deduplication
	msg.Header.Set("Nats-Msg-Id", event.EventID)

	// Publish synchronously, expecting JetStream ack
	ack, err := c.js.PublishMsg(ctx, msg)
	if err != nil {
		observability.RecordSpanError(span, err)
		m.OutboxPublishFailuresTotal.WithLabelValues(event.EventType).Inc()
		return fmt.Errorf("failed to publish to jetstream: %w", err)
	}

	m.OutboxPublishTotal.WithLabelValues(event.EventType).Inc()

	slog.DebugContext(ctx, "published event to NATS",
		slog.String("event_id", event.EventID),
		slog.String("subject", subject),
		slog.Uint64("seq", ack.Sequence),
	)
	return nil
}

func (c *NATSClient) JetStream() jetstream.JetStream {
	return c.js
}

func (c *NATSClient) Close() {
	if c.nc != nil {
		c.nc.Drain()
	}
}
