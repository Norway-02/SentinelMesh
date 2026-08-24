package audit

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
)

// PostgresRepository implements durable audit event storage in PostgreSQL.
type PostgresRepository struct {
	db *sql.DB
}

// NewPostgresRepository constructs a PostgresRepository.
func NewPostgresRepository(db *sql.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

// Insert writes a security audit event to PostgreSQL.
func (r *PostgresRepository) Insert(ctx context.Context, e AuditEvent) error {
	var metaJSON []byte
	var err error
	if e.Metadata != nil {
		metaJSON, err = json.Marshal(e.Metadata)
		if err != nil {
			return fmt.Errorf("failed to marshal metadata: %w", err)
		}
	}

	query := `
		INSERT INTO security_audit_events (
			id, run_id, agent_id, tenant_id, correlation_id, source,
			event_type, operation, resource, decision, rule_id, reason,
			severity, occurred_at, metadata
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15
		)`

	_, err = r.db.ExecContext(ctx, query,
		e.ID, e.RunID, e.AgentID, e.TenantID, e.CorrelationID, e.Source,
		e.EventType, e.Operation, e.Resource, e.Decision, e.RuleID, e.Reason,
		e.Severity, e.OccurredAt, metaJSON,
	)
	if err != nil {
		return fmt.Errorf("failed to insert security audit event: %w", err)
	}

	return nil
}

// GetByRunID fetches all audit events for a given run ID ordered by occurrence.
func (r *PostgresRepository) GetByRunID(ctx context.Context, runID string) ([]AuditEvent, error) {
	return r.List(ctx, AuditFilter{RunID: runID})
}

// List queries audit events based on filtering criteria.
func (r *PostgresRepository) List(ctx context.Context, filter AuditFilter) ([]AuditEvent, error) {
	var conditions []string
	var args []interface{}
	argIdx := 1

	if filter.RunID != "" {
		conditions = append(conditions, fmt.Sprintf("run_id = $%d", argIdx))
		args = append(args, filter.RunID)
		argIdx++
	}
	if filter.AgentID != "" {
		conditions = append(conditions, fmt.Sprintf("agent_id = $%d", argIdx))
		args = append(args, filter.AgentID)
		argIdx++
	}
	if filter.TenantID != "" {
		conditions = append(conditions, fmt.Sprintf("tenant_id = $%d", argIdx))
		args = append(args, filter.TenantID)
		argIdx++
	}
	if filter.Decision != "" {
		conditions = append(conditions, fmt.Sprintf("decision = $%d", argIdx))
		args = append(args, filter.Decision)
		argIdx++
	}
	if filter.Severity != "" {
		conditions = append(conditions, fmt.Sprintf("severity = $%d", argIdx))
		args = append(args, filter.Severity)
		argIdx++
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	limitClause := "LIMIT 100"
	if filter.Limit > 0 {
		limitClause = fmt.Sprintf("LIMIT %d", filter.Limit)
	}

	query := fmt.Sprintf(`
		SELECT id, run_id, agent_id, tenant_id, correlation_id, source,
		       event_type, operation, resource, decision, rule_id, reason,
		       severity, occurred_at, metadata
		FROM security_audit_events
		%s
		ORDER BY occurred_at DESC
		%s`, whereClause, limitClause)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query security audit events: %w", err)
	}
	defer rows.Close()

	var events []AuditEvent
	for rows.Next() {
		var e AuditEvent
		var metaJSON []byte

		err := rows.Scan(
			&e.ID, &e.RunID, &e.AgentID, &e.TenantID, &e.CorrelationID, &e.Source,
			&e.EventType, &e.Operation, &e.Resource, &e.Decision, &e.RuleID, &e.Reason,
			&e.Severity, &e.OccurredAt, &metaJSON,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan audit row: %w", err)
		}

		if len(metaJSON) > 0 {
			_ = json.Unmarshal(metaJSON, &e.Metadata)
		}
		events = append(events, e)
	}

	return events, nil
}
