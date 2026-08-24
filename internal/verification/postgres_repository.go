package verification

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/sentinelmesh/sentinelmesh/internal/repository/postgres"
)

// PostgresRepository provides PostgreSQL persistence for attestation records.
type PostgresRepository struct {
	db postgres.DBExecutor
}

// NewPostgresRepository constructs a PostgresRepository.
func NewPostgresRepository(db postgres.DBExecutor) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) Save(ctx context.Context, record AttestationRecord) error {
	evalJSON, err := json.Marshal(record.Evaluations)
	if err != nil {
		return fmt.Errorf("failed to marshal rule evaluations: %w", err)
	}

	query := `
		INSERT INTO agent_attestation_records (
			id, run_id, agent_id, tenant_id, status, evidence_digest, rule_evaluations, attested_at, created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9
		)
	`
	db := postgres.ExtractDB(ctx, r.db)
	_, err = db.ExecContext(ctx, query,
		record.ID,
		record.RunID,
		record.AgentID,
		record.TenantID,
		string(record.Status),
		record.EvidenceDigest,
		evalJSON,
		record.AttestedAt,
		record.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert attestation record: %w", err)
	}

	return nil
}

func (r *PostgresRepository) GetByID(ctx context.Context, id string) (*AttestationRecord, error) {
	query := `
		SELECT id, run_id, agent_id, tenant_id, status, evidence_digest, rule_evaluations, attested_at, created_at
		FROM agent_attestation_records
		WHERE id = $1
	`
	db := postgres.ExtractDB(ctx, r.db)
	row := db.QueryRowContext(ctx, query, id)

	var rec AttestationRecord
	var statusStr string
	var evalJSON []byte

	err := row.Scan(
		&rec.ID,
		&rec.RunID,
		&rec.AgentID,
		&rec.TenantID,
		&statusStr,
		&rec.EvidenceDigest,
		&evalJSON,
		&rec.AttestedAt,
		&rec.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrAttestationNotFound
		}
		return nil, fmt.Errorf("failed to query attestation record: %w", err)
	}

	rec.Status = AttestationStatus(statusStr)
	if err := json.Unmarshal(evalJSON, &rec.Evaluations); err != nil {
		return nil, fmt.Errorf("failed to unmarshal evaluations: %w", err)
	}

	return &rec, nil
}

func (r *PostgresRepository) GetByRunID(ctx context.Context, runID string) (*AttestationRecord, error) {
	query := `
		SELECT id, run_id, agent_id, tenant_id, status, evidence_digest, rule_evaluations, attested_at, created_at
		FROM agent_attestation_records
		WHERE run_id = $1
		ORDER BY attested_at DESC
		LIMIT 1
	`
	db := postgres.ExtractDB(ctx, r.db)
	row := db.QueryRowContext(ctx, query, runID)

	var rec AttestationRecord
	var statusStr string
	var evalJSON []byte

	err := row.Scan(
		&rec.ID,
		&rec.RunID,
		&rec.AgentID,
		&rec.TenantID,
		&statusStr,
		&rec.EvidenceDigest,
		&evalJSON,
		&rec.AttestedAt,
		&rec.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrAttestationNotFound
		}
		return nil, fmt.Errorf("failed to query attestation record by run: %w", err)
	}

	rec.Status = AttestationStatus(statusStr)
	if err := json.Unmarshal(evalJSON, &rec.Evaluations); err != nil {
		return nil, fmt.Errorf("failed to unmarshal evaluations: %w", err)
	}

	return &rec, nil
}
