package postgres

import (
	"context"
	"database/sql"
	"strings"

	"github.com/google/uuid"
	"github.com/sentinelmesh/sentinelmesh/internal/domain"
	"github.com/sentinelmesh/sentinelmesh/internal/repository"
)

type RunRepository struct {
	db *sql.DB
}

func NewRunRepository(db *sql.DB) *RunRepository {
	return &RunRepository{db: db}
}

func (r *RunRepository) Create(ctx context.Context, run domain.AgentRun) error {
	if run.Version == 0 {
		run.Version = 1
	}

	query := `
		INSERT INTO agent_runs (
			id, agent_id, tenant_id, state, attempt, retry_count,
			cluster, node, last_checkpoint_id, failure_reason, verification_state,
			fencing_token, execution_generation,
			version, started_at, finished_at, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10, $11,
			$12, $13,
			$14, $15, $16, $17, $18
		)
	`
	db := extractDB(ctx, r.db)
	_, err := db.ExecContext(ctx, query,
		run.ID, run.AgentID, run.TenantID, run.State, run.Attempt, run.RetryCount,
		run.Cluster, run.Node, run.LastCheckpointID, run.FailureReason, run.VerificationState,
		run.FencingToken, run.RecoveryGeneration,
		run.Version, run.StartedAt, run.FinishedAt, run.CreatedAt, run.UpdatedAt,
	)

	if err != nil {
		if strings.Contains(err.Error(), "SQLSTATE 23505") {
			return repository.ErrAlreadyExists
		}
		return err
	}
	return nil
}

func (r *RunRepository) Get(ctx context.Context, id string) (domain.AgentRun, error) {
	query := `
		SELECT 
			id, agent_id, tenant_id, state, attempt, retry_count,
			cluster, node, last_checkpoint_id, failure_reason, verification_state,
			fencing_token, execution_generation,
			version, started_at, finished_at, created_at, updated_at
		FROM agent_runs
		WHERE id = $1
	`
	db := extractDB(ctx, r.db)
	row := db.QueryRowContext(ctx, query, id)

	var run domain.AgentRun
	err := row.Scan(
		&run.ID, &run.AgentID, &run.TenantID, &run.State, &run.Attempt, &run.RetryCount,
		&run.Cluster, &run.Node, &run.LastCheckpointID, &run.FailureReason, &run.VerificationState,
		&run.FencingToken, &run.RecoveryGeneration,
		&run.Version, &run.StartedAt, &run.FinishedAt, &run.CreatedAt, &run.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return domain.AgentRun{}, repository.ErrNotFound
		}
		return domain.AgentRun{}, err
	}

	return run, nil
}

func (r *RunRepository) Update(ctx context.Context, run domain.AgentRun) error {
	db := extractDB(ctx, r.db)

	// Fetch old state before updating, since we need from_state
	var oldState string
	err := db.QueryRowContext(ctx, "SELECT state FROM agent_runs WHERE id = $1 FOR UPDATE", run.ID).Scan(&oldState)
	if err != nil && err != sql.ErrNoRows {
		return err
	}

	newVersion := run.Version + 1

	query := `
		UPDATE agent_runs SET
			state = $1,
			attempt = $2,
			retry_count = $3,
			cluster = $4,
			node = $5,
			last_checkpoint_id = $6,
			failure_reason = $7,
			verification_state = $8,
			fencing_token = $9,
			execution_generation = $10,
			version = $11,
			started_at = $12,
			finished_at = $13,
			updated_at = $14
		WHERE id = $15 AND version = $16
	`
	res, err := db.ExecContext(ctx, query,
		run.State, run.Attempt, run.RetryCount,
		run.Cluster, run.Node, run.LastCheckpointID,
		run.FailureReason, run.VerificationState,
		run.FencingToken, run.RecoveryGeneration,
		newVersion, run.StartedAt, run.FinishedAt, run.UpdatedAt,
		run.ID, run.Version,
	)
	if err != nil {
		return err
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return repository.ErrConflict
	}

	// Add state history record
	historyQuery := `
		INSERT INTO agent_run_state_history (
			id, run_id, from_state, to_state, timestamp
		) VALUES (
			$1, $2, $3, $4, $5
		)
	`
	_, err = db.ExecContext(ctx, historyQuery,
		uuid.NewString(), run.ID, oldState, run.State, run.UpdatedAt,
	)
	if err != nil {
		return err
	}

	return nil
}

func (r *RunRepository) ListByNode(ctx context.Context, nodeID string) ([]domain.AgentRun, error) {
	query := `
		SELECT 
			id, agent_id, tenant_id, state, attempt, retry_count,
			cluster, node, last_checkpoint_id, failure_reason, verification_state,
			fencing_token, execution_generation,
			version, started_at, finished_at, created_at, updated_at
		FROM agent_runs
		WHERE node = $1
	`
	db := extractDB(ctx, r.db)
	rows, err := db.QueryContext(ctx, query, nodeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var runs []domain.AgentRun
	for rows.Next() {
		var run domain.AgentRun
		err := rows.Scan(
			&run.ID, &run.AgentID, &run.TenantID, &run.State, &run.Attempt, &run.RetryCount,
			&run.Cluster, &run.Node, &run.LastCheckpointID, &run.FailureReason, &run.VerificationState,
			&run.FencingToken, &run.RecoveryGeneration,
			&run.Version, &run.StartedAt, &run.FinishedAt, &run.CreatedAt, &run.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}

	return runs, nil
}

func (r *RunRepository) ListByCluster(ctx context.Context, clusterID string) ([]domain.AgentRun, error) {
	query := `
		SELECT 
			id, agent_id, tenant_id, state, attempt, retry_count,
			cluster, node, last_checkpoint_id, failure_reason, verification_state,
			fencing_token, execution_generation,
			version, started_at, finished_at, created_at, updated_at
		FROM agent_runs
		WHERE cluster = $1
	`
	db := extractDB(ctx, r.db)
	rows, err := db.QueryContext(ctx, query, clusterID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var runs []domain.AgentRun
	for rows.Next() {
		var run domain.AgentRun
		err := rows.Scan(
			&run.ID, &run.AgentID, &run.TenantID, &run.State, &run.Attempt, &run.RetryCount,
			&run.Cluster, &run.Node, &run.LastCheckpointID, &run.FailureReason, &run.VerificationState,
			&run.FencingToken, &run.RecoveryGeneration,
			&run.Version, &run.StartedAt, &run.FinishedAt, &run.CreatedAt, &run.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}

	return runs, nil
}
