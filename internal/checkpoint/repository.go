package checkpoint

import (
	"context"
)

// Repository defines operations for persisting and querying checkpoints.
type Repository interface {
	// Save stores a checkpoint. Must be idempotent on (run_id, sequence_number, checksum).
	// Returns ErrSequenceConflict if (run_id, sequence_number) exists with different checksum.
	Save(ctx context.Context, cp Checkpoint) error

	// GetLatest returns the newest checkpoint for a run (highest sequence_number).
	GetLatest(ctx context.Context, runID string) (*Checkpoint, error)

	// GetBySequence returns the checkpoint for a run with the exact sequence number.
	GetBySequence(ctx context.Context, runID string, seq int64) (*Checkpoint, error)

	// GetByID fetches a checkpoint by primary key ID.
	GetByID(ctx context.Context, id string) (*Checkpoint, error)

	// List returns all checkpoints for a run ordered by sequence_number ascending.
	List(ctx context.Context, runID string) ([]Checkpoint, error)

	// DeleteAll removes all checkpoints for a run (e.g. on run cleanup).
	DeleteAll(ctx context.Context, runID string) error
}
