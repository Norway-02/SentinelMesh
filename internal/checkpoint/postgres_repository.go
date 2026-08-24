package checkpoint

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
)

// PostgresRepository implements Checkpoint persistence in PostgreSQL.
type PostgresRepository struct {
	db *sql.DB
}

// NewPostgresRepository constructs a PostgresRepository.
func NewPostgresRepository(db *sql.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

// Save stores a checkpoint, handling idempotency on identical checksums.
func (r *PostgresRepository) Save(ctx context.Context, cp Checkpoint) error {
	if err := cp.Validate(); err != nil {
		return err
	}

	var metaJSON []byte
	var err error
	if cp.Metadata != nil {
		metaJSON, err = json.Marshal(cp.Metadata)
		if err != nil {
			return fmt.Errorf("failed to marshal metadata: %w", err)
		}
	}

	// 1. Check if checkpoint with same sequence exists
	var existingChecksum string
	err = r.db.QueryRowContext(ctx, `
		SELECT state_checksum FROM agent_checkpoints
		WHERE run_id = $1 AND sequence_number = $2
	`, cp.RunID, cp.SequenceNumber).Scan(&existingChecksum)

	if err == nil {
		if existingChecksum == cp.StateChecksum {
			// Idempotent retry: exact same checkpoint already committed
			return nil
		}
		// Sequence conflict: same sequence number with different state
		return ErrSequenceConflict
	} else if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("failed to check existing checkpoint: %w", err)
	}

	// 2. Insert new checkpoint
	query := `
		INSERT INTO agent_checkpoints (
			id, run_id, agent_id, tenant_id, sequence_number,
			state_inline, state_uri, state_checksum, size_bytes,
			metadata, created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
		)`

	var stateInlineVal interface{}
	if len(cp.StateInline) > 0 {
		stateInlineVal = cp.StateInline
	}

	var stateURIVal interface{}
	if cp.StateURI != "" {
		stateURIVal = cp.StateURI
	}

	_, err = r.db.ExecContext(ctx, query,
		cp.ID, cp.RunID, cp.AgentID, cp.TenantID, cp.SequenceNumber,
		stateInlineVal, stateURIVal, cp.StateChecksum, cp.SizeBytes,
		metaJSON, cp.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert checkpoint: %w", err)
	}

	return nil
}

// GetLatest returns the highest sequence number checkpoint for a run.
func (r *PostgresRepository) GetLatest(ctx context.Context, runID string) (*Checkpoint, error) {
	query := `
		SELECT id, run_id, agent_id, tenant_id, sequence_number,
		       state_inline, state_uri, state_checksum, size_bytes,
		       metadata, created_at
		FROM agent_checkpoints
		WHERE run_id = $1
		ORDER BY sequence_number DESC
		LIMIT 1`

	return r.scanCheckpoint(r.db.QueryRowContext(ctx, query, runID))
}

// GetBySequence returns a specific sequence checkpoint.
func (r *PostgresRepository) GetBySequence(ctx context.Context, runID string, seq int64) (*Checkpoint, error) {
	query := `
		SELECT id, run_id, agent_id, tenant_id, sequence_number,
		       state_inline, state_uri, state_checksum, size_bytes,
		       metadata, created_at
		FROM agent_checkpoints
		WHERE run_id = $1 AND sequence_number = $2`

	return r.scanCheckpoint(r.db.QueryRowContext(ctx, query, runID, seq))
}

// GetByID returns a checkpoint by primary key.
func (r *PostgresRepository) GetByID(ctx context.Context, id string) (*Checkpoint, error) {
	query := `
		SELECT id, run_id, agent_id, tenant_id, sequence_number,
		       state_inline, state_uri, state_checksum, size_bytes,
		       metadata, created_at
		FROM agent_checkpoints
		WHERE id = $1`

	return r.scanCheckpoint(r.db.QueryRowContext(ctx, query, id))
}

// List returns all checkpoints for a run ordered by sequence number ascending.
func (r *PostgresRepository) List(ctx context.Context, runID string) ([]Checkpoint, error) {
	query := `
		SELECT id, run_id, agent_id, tenant_id, sequence_number,
		       state_inline, state_uri, state_checksum, size_bytes,
		       metadata, created_at
		FROM agent_checkpoints
		WHERE run_id = $1
		ORDER BY sequence_number ASC`

	rows, err := r.db.QueryContext(ctx, query, runID)
	if err != nil {
		return nil, fmt.Errorf("failed to query checkpoints: %w", err)
	}
	defer rows.Close()

	var checkpoints []Checkpoint
	for rows.Next() {
		var cp Checkpoint
		var stateInline sql.NullString
		var stateURI sql.NullString
		var metaJSON []byte

		err := rows.Scan(
			&cp.ID, &cp.RunID, &cp.AgentID, &cp.TenantID, &cp.SequenceNumber,
			&stateInline, &stateURI, &cp.StateChecksum, &cp.SizeBytes,
			&metaJSON, &cp.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan checkpoint row: %w", err)
		}

		if stateInline.Valid {
			cp.StateInline = json.RawMessage(stateInline.String)
		}
		if stateURI.Valid {
			cp.StateURI = stateURI.String
		}
		if len(metaJSON) > 0 {
			_ = json.Unmarshal(metaJSON, &cp.Metadata)
		}

		if !cp.VerifyIntegrity() {
			return nil, ErrCorruptedCheckpoint
		}

		checkpoints = append(checkpoints, cp)
	}

	return checkpoints, nil
}

// DeleteAll removes all checkpoints for a run.
func (r *PostgresRepository) DeleteAll(ctx context.Context, runID string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM agent_checkpoints WHERE run_id = $1`, runID)
	if err != nil {
		return fmt.Errorf("failed to delete checkpoints: %w", err)
	}
	return nil
}

func (r *PostgresRepository) scanCheckpoint(row *sql.Row) (*Checkpoint, error) {
	var cp Checkpoint
	var stateInline sql.NullString
	var stateURI sql.NullString
	var metaJSON []byte

	err := row.Scan(
		&cp.ID, &cp.RunID, &cp.AgentID, &cp.TenantID, &cp.SequenceNumber,
		&stateInline, &stateURI, &cp.StateChecksum, &cp.SizeBytes,
		&metaJSON, &cp.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrCheckpointNotFound
		}
		return nil, fmt.Errorf("failed to scan checkpoint: %w", err)
	}

	if stateInline.Valid {
		cp.StateInline = json.RawMessage(stateInline.String)
	}
	if stateURI.Valid {
		cp.StateURI = stateURI.String
	}
	if len(metaJSON) > 0 {
		_ = json.Unmarshal(metaJSON, &cp.Metadata)
	}

	if !cp.VerifyIntegrity() {
		return nil, ErrCorruptedCheckpoint
	}

	return &cp, nil
}
