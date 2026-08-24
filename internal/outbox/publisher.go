package outbox

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"

	"github.com/sentinelmesh/sentinelmesh/internal/messaging"
	"github.com/sentinelmesh/sentinelmesh/internal/observability"
)

type Publisher struct {
	repo       Repository
	natsClient *messaging.NATSClient
	pollTicker *time.Ticker
	quit       chan struct{}
	
	publisherID string
}

func NewPublisher(repo Repository, natsClient *messaging.NATSClient, pollInterval time.Duration) *Publisher {
	return &Publisher{
		repo:        repo,
		natsClient:  natsClient,
		pollTicker:  time.NewTicker(pollInterval),
		quit:        make(chan struct{}),
		publisherID: uuid.NewString(), // unique claim owner ID
	}
}

func (p *Publisher) Start() {
	go p.run()
}

func (p *Publisher) Stop() {
	close(p.quit)
	p.pollTicker.Stop()
}

func (p *Publisher) run() {
	slog.Info("Outbox publisher started", slog.String("publisher_id", p.publisherID))
	for {
		select {
		case <-p.pollTicker.C:
			p.processBatch(context.Background())
		case <-p.quit:
			slog.Info("Outbox publisher stopped")
			return
		}
	}
}

func (p *Publisher) processBatch(ctx context.Context) {
	m := observability.GetMetrics()

	// We use a 30s claim duration. If we crash, another publisher picks it up after 30s.
	events, err := p.repo.Claim(ctx, 100, p.publisherID, 30*time.Second)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to claim outbox events", slog.Any("error", err))
		return
	}

	if len(events) == 0 {
		return
	}

	m.OutboxPendingEvents.WithLabelValues("Run").Set(float64(len(events)))

	slog.DebugContext(ctx, "Claimed events for publishing", slog.Int("count", len(events)))

	for _, evt := range events {
		evtCtx := ctx
		if evt.TraceParent != "" {
			hdr := make(http.Header)
			hdr.Set("traceparent", evt.TraceParent)
			evtCtx = observability.ExtractHTTPHeaders(evtCtx, hdr)
		}
		evtCtx = observability.WithRunID(evtCtx, evt.AggregateID)
		evtCtx = observability.WithTenantID(evtCtx, evt.TenantID)
		evtCtx = observability.WithCorrelationID(evtCtx, evt.CorrelationID)

		evtCtx, span := observability.StartSpan(evtCtx, "outbox.publish")
		observability.InjectSpanAttributes(span, evt.AggregateID, "", evt.TenantID, evt.CorrelationID, 0)
		span.SetAttributes(
			attribute.String("outbox.event_id", evt.EventID),
			attribute.String("outbox.event_type", evt.EventType),
		)

		// Attempt to publish
		err := p.natsClient.Publish(evtCtx, evt.EventType, evt)
		if err != nil {
			observability.RecordSpanError(span, err)
			span.End()
			slog.ErrorContext(evtCtx, "Failed to publish event to NATS",
				slog.String("event_id", evt.EventID),
				slog.Any("error", err),
			)
			// Mark as failed in outbox (releases claim, increments attempts)
			if mErr := p.repo.MarkFailed(ctx, evt.EventID, err.Error()); mErr != nil {
				slog.ErrorContext(evtCtx, "Failed to update outbox error state", slog.Any("error", mErr))
			}
			continue
		}

		// Mark as published
		if mErr := p.repo.MarkPublished(ctx, evt.EventID); mErr != nil {
			observability.RecordSpanError(span, mErr)
			slog.ErrorContext(evtCtx, "Failed to mark outbox event as published",
				slog.String("event_id", evt.EventID),
				slog.Any("error", mErr),
			)
		}
		span.End()
	}
}
