package checkpoint

import (
	"context"
	"sort"
	"sync"
)

// MemoryRepository is an in-memory implementation of checkpoint.Repository.
type MemoryRepository struct {
	mu          sync.RWMutex
	checkpoints map[string][]Checkpoint // keyed by runID
	byID        map[string]Checkpoint
}

// NewMemoryRepository constructs a MemoryRepository.
func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		checkpoints: make(map[string][]Checkpoint),
		byID:        make(map[string]Checkpoint),
	}
}

// Save stores a checkpoint, guaranteeing idempotency and sequence conflict detection.
func (r *MemoryRepository) Save(ctx context.Context, cp Checkpoint) error {
	if err := cp.Validate(); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	list := r.checkpoints[cp.RunID]

	for _, existing := range list {
		if existing.SequenceNumber == cp.SequenceNumber {
			if existing.StateChecksum == cp.StateChecksum {
				// Idempotent retry: identical checkpoint already exists
				return nil
			}
			// Sequence conflict: same sequence number with different state payload
			return ErrSequenceConflict
		}
	}

	r.checkpoints[cp.RunID] = append(list, cp)
	r.byID[cp.ID] = cp

	// Keep sorted by sequence number ascending
	sort.Slice(r.checkpoints[cp.RunID], func(i, j int) bool {
		return r.checkpoints[cp.RunID][i].SequenceNumber < r.checkpoints[cp.RunID][j].SequenceNumber
	})

	return nil
}

// SaveRaw stores a checkpoint directly without validation, enabling bit-rot / corruption simulation in tests.
func (r *MemoryRepository) SaveRaw(ctx context.Context, cp Checkpoint) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.checkpoints[cp.RunID] = append(r.checkpoints[cp.RunID], cp)
	r.byID[cp.ID] = cp
	return nil
}

// GetLatest returns the highest sequence number checkpoint for a run.
func (r *MemoryRepository) GetLatest(ctx context.Context, runID string) (*Checkpoint, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	list, exists := r.checkpoints[runID]
	if !exists || len(list) == 0 {
		return nil, ErrCheckpointNotFound
	}

	latest := list[len(list)-1]
	if !latest.VerifyIntegrity() {
		return nil, ErrCorruptedCheckpoint
	}

	cpCopy := latest
	return &cpCopy, nil
}

// GetBySequence returns a specific sequence checkpoint for a run.
func (r *MemoryRepository) GetBySequence(ctx context.Context, runID string, seq int64) (*Checkpoint, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	list, exists := r.checkpoints[runID]
	if !exists {
		return nil, ErrCheckpointNotFound
	}

	for _, cp := range list {
		if cp.SequenceNumber == seq {
			if !cp.VerifyIntegrity() {
				return nil, ErrCorruptedCheckpoint
			}
			cpCopy := cp
			return &cpCopy, nil
		}
	}

	return nil, ErrCheckpointNotFound
}

// GetByID retrieves a checkpoint by ID.
func (r *MemoryRepository) GetByID(ctx context.Context, id string) (*Checkpoint, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	cp, exists := r.byID[id]
	if !exists {
		return nil, ErrCheckpointNotFound
	}

	if !cp.VerifyIntegrity() {
		return nil, ErrCorruptedCheckpoint
	}

	cpCopy := cp
	return &cpCopy, nil
}

// List returns all checkpoints for a run ordered by sequence number ascending.
func (r *MemoryRepository) List(ctx context.Context, runID string) ([]Checkpoint, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	list, exists := r.checkpoints[runID]
	if !exists {
		return []Checkpoint{}, nil
	}

	res := make([]Checkpoint, len(list))
	copy(res, list)
	return res, nil
}

// DeleteAll removes all checkpoints for a run.
func (r *MemoryRepository) DeleteAll(ctx context.Context, runID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if list, exists := r.checkpoints[runID]; exists {
		for _, cp := range list {
			delete(r.byID, cp.ID)
		}
		delete(r.checkpoints, runID)
	}

	return nil
}
