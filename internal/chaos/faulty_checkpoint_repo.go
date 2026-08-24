package chaos

import (
	"context"
	"errors"
	"time"

	"github.com/sentinelmesh/sentinelmesh/internal/checkpoint"
)

// FaultyCheckpointRepository wraps a checkpoint.Repository to inject corrupted or truncated state.
type FaultyCheckpointRepository struct {
	underlying checkpoint.Repository
	controller *FaultController
}

// NewFaultyCheckpointRepository constructs a FaultyCheckpointRepository wrapper.
func NewFaultyCheckpointRepository(underlying checkpoint.Repository, controller *FaultController) *FaultyCheckpointRepository {
	return &FaultyCheckpointRepository{
		underlying: underlying,
		controller: controller,
	}
}

// Save persists a checkpoint unless intercepted by a fault.
func (r *FaultyCheckpointRepository) Save(ctx context.Context, cp checkpoint.Checkpoint) error {
	if trigger, spec := r.controller.ShouldTrigger("Save"); trigger {
		if spec.Delay > 0 {
			time.Sleep(spec.Delay)
		}
		if spec.Type == FaultError {
			errMsg := spec.ErrorMsg
			if errMsg == "" {
				errMsg = "simulated checkpoint disk write failure"
			}
			return errors.New(errMsg)
		}
	}
	return r.underlying.Save(ctx, cp)
}

// GetLatest returns the newest checkpoint for a run, potentially injecting corruption or truncation.
func (r *FaultyCheckpointRepository) GetLatest(ctx context.Context, runID string) (*checkpoint.Checkpoint, error) {
	if trigger, spec := r.controller.ShouldTrigger("GetLatest"); trigger {
		if spec.Delay > 0 {
			time.Sleep(spec.Delay)
		}
		if spec.Type == FaultError {
			errMsg := spec.ErrorMsg
			if errMsg == "" {
				errMsg = "simulated checkpoint storage read failure"
			}
			return nil, errors.New(errMsg)
		}

		cp, err := r.underlying.GetLatest(ctx, runID)
		if err != nil {
			return nil, err
		}
		if cp == nil {
			return nil, checkpoint.ErrCheckpointNotFound
		}

		// Inject Checksum Corruption (F09)
		if spec.Type == FaultCorrupt {
			corrupted := *cp
			corrupted.StateChecksum = "corrupted-crc32-mismatch"
			return &corrupted, nil
		}

		// Inject Partial / Truncated Payload (F10)
		if spec.Type == FaultTruncate {
			truncated := *cp
			truncated.StateInline = []byte(`{"step":25,"truncated":true,"partial_data":`)
			truncated.SizeBytes = int64(len(truncated.StateInline))
			// StateChecksum doesn't match truncated bytes either
			return &truncated, nil
		}
	}

	return r.underlying.GetLatest(ctx, runID)
}

// GetBySequence returns a specific sequence checkpoint for a run.
func (r *FaultyCheckpointRepository) GetBySequence(ctx context.Context, runID string, seq int64) (*checkpoint.Checkpoint, error) {
	if trigger, spec := r.controller.ShouldTrigger("GetBySequence"); trigger {
		if spec.Type == FaultError {
			return nil, errors.New("simulated checkpoint lookup error")
		}
	}
	return r.underlying.GetBySequence(ctx, runID, seq)
}

// GetByID fetches a checkpoint by primary key ID.
func (r *FaultyCheckpointRepository) GetByID(ctx context.Context, id string) (*checkpoint.Checkpoint, error) {
	if trigger, spec := r.controller.ShouldTrigger("GetByID"); trigger {
		if spec.Type == FaultError {
			return nil, errors.New("simulated checkpoint lookup error")
		}
	}
	return r.underlying.GetByID(ctx, id)
}

// List returns all checkpoints for a run.
func (r *FaultyCheckpointRepository) List(ctx context.Context, runID string) ([]checkpoint.Checkpoint, error) {
	return r.underlying.List(ctx, runID)
}

// DeleteAll removes all checkpoints for a run.
func (r *FaultyCheckpointRepository) DeleteAll(ctx context.Context, runID string) error {
	return r.underlying.DeleteAll(ctx, runID)
}
