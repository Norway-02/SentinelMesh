package postgres

import (
	"context"
	"database/sql"
	"time"

	"github.com/sentinelmesh/sentinelmesh/internal/events"
)

type OutboxRepository struct {
	db *sql.DB
}

func NewOutboxRepository(db *sql.DB) *OutboxRepository {
	return &OutboxRepository{db: db}
}

func (r *OutboxRepository) Insert(ctx context.Context, event events.Event) error {
	query := `
		INSERT INTO outbox_events (
			id, aggregate_type, aggregate_id, event_type, schema_version,
			tenant_id, correlation_id, payload, created_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9
		)
	`
	db := extractDB(ctx, r.db)
	_, err := db.ExecContext(ctx, query,
		event.EventID, event.AggregateType, event.AggregateID, event.EventType, event.SchemaVersion,
		event.TenantID, event.CorrelationID, event.Payload, event.OccurredAt,
	)
	return err
}

func (r *OutboxRepository) Claim(ctx context.Context, batchSize int, ownerID string, claimDuration time.Duration) ([]events.Event, error) {
	// Claim events that are unpublished and either never claimed, or the claim has expired
	query := `
		UPDATE outbox_events
		SET claimed_at = NOW(), claim_owner = $1
		WHERE id IN (
			SELECT id FROM outbox_events
			WHERE published_at IS NULL
			  AND (claimed_at IS NULL OR claimed_at < NOW() - $2::interval)
			ORDER BY created_at ASC
			LIMIT $3
			FOR UPDATE SKIP LOCKED
		)
		RETURNING 
			id, aggregate_type, aggregate_id, event_type, schema_version,
			tenant_id, correlation_id, payload, created_at
	`
	
	intervalStr := claimDuration.String()
	
	rows, err := r.db.QueryContext(ctx, query, ownerID, intervalStr, batchSize)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var evts []events.Event
	for rows.Next() {
		var e events.Event
		var corrID sql.NullString
		err := rows.Scan(
			&e.EventID, &e.AggregateType, &e.AggregateID, &e.EventType, &e.SchemaVersion,
			&e.TenantID, &corrID, &e.Payload, &e.OccurredAt,
		)
		if err != nil {
			return nil, err
		}
		if corrID.Valid {
			e.CorrelationID = corrID.String
		}
		evts = append(evts, e)
	}
	return evts, rows.Err()
}

func (r *OutboxRepository) MarkPublished(ctx context.Context, eventID string) error {
	query := `UPDATE outbox_events SET published_at = NOW(), claim_owner = NULL WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, eventID)
	return err
}

func (r *OutboxRepository) MarkFailed(ctx context.Context, eventID string, errStr string) error {
	query := `
		UPDATE outbox_events 
		SET attempts = attempts + 1, last_error = $1, claim_owner = NULL
		WHERE id = $2
	`
	_, err := r.db.ExecContext(ctx, query, errStr, eventID)
	return err
}
