package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/sentinelmesh/sentinelmesh/internal/domain"
	"github.com/sentinelmesh/sentinelmesh/internal/repository"
)

type assignmentRepository struct {
	db DBExecutor
}

func NewAssignmentRepository(db DBExecutor) repository.AssignmentRepository {
	return &assignmentRepository{db: db}
}

func (r *assignmentRepository) Assign(ctx context.Context, assignment *domain.SchedulingAssignment) (bool, error) {
	db := extractDB(ctx, r.db)

	decisionJSON, err := json.Marshal(assignment.Decision)
	if err != nil {
		return false, fmt.Errorf("failed to marshal decision: %w", err)
	}

	query := `
		INSERT INTO run_scheduling_assignments (
			run_id, cluster_id, node_id, algorithm_version, execution_generation, fencing_token, score, decision, created_at, updated_at, version
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
		) ON CONFLICT (run_id) DO NOTHING
	`

	res, err := db.ExecContext(ctx, query,
		assignment.RunID,
		assignment.ClusterID,
		assignment.NodeID,
		assignment.AlgorithmVersion,
		assignment.ExecutionGeneration,
		assignment.FencingToken,
		assignment.Score,
		decisionJSON,
		assignment.CreatedAt,
		assignment.UpdatedAt,
		assignment.Version,
	)
	if err != nil {
		return false, fmt.Errorf("failed to insert assignment: %w", err)
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		var existingNode string
		err := db.QueryRowContext(ctx, "SELECT node_id FROM run_scheduling_assignments WHERE run_id = $1", assignment.RunID).Scan(&existingNode)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return false, fmt.Errorf("failed to fetch existing assignment: %w", err)
		}

		if existingNode != assignment.NodeID {
			return false, fmt.Errorf("conflict: run %s is already assigned to %s (attempted %s)", assignment.RunID, existingNode, assignment.NodeID)
		}

		return false, nil
	}

	return true, nil
}

func (r *assignmentRepository) Reassign(ctx context.Context, assignment *domain.SchedulingAssignment) error {
	db := extractDB(ctx, r.db)

	decisionJSON, err := json.Marshal(assignment.Decision)
	if err != nil {
		return fmt.Errorf("failed to marshal decision: %w", err)
	}

	query := `
		INSERT INTO run_scheduling_assignments (
			run_id, cluster_id, node_id, algorithm_version, execution_generation, fencing_token, score, decision, created_at, updated_at, version
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
		) ON CONFLICT (run_id) DO UPDATE SET
			cluster_id = EXCLUDED.cluster_id,
			node_id = EXCLUDED.node_id,
			algorithm_version = EXCLUDED.algorithm_version,
			execution_generation = EXCLUDED.execution_generation,
			fencing_token = EXCLUDED.fencing_token,
			score = EXCLUDED.score,
			decision = EXCLUDED.decision,
			updated_at = EXCLUDED.updated_at,
			version = run_scheduling_assignments.version + 1
	`

	_, err = db.ExecContext(ctx, query,
		assignment.RunID,
		assignment.ClusterID,
		assignment.NodeID,
		assignment.AlgorithmVersion,
		assignment.ExecutionGeneration,
		assignment.FencingToken,
		assignment.Score,
		decisionJSON,
		assignment.CreatedAt,
		assignment.UpdatedAt,
		assignment.Version,
	)
	if err != nil {
		return fmt.Errorf("failed to reassign run: %w", err)
	}

	return nil
}
