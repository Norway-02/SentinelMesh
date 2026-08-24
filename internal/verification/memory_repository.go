package verification

import (
	"context"
	"sync"
)

// MemoryRepository provides an in-memory attestation record store.
type MemoryRepository struct {
	mu      sync.RWMutex
	records map[string]AttestationRecord
	byRunID map[string]string // runID -> attestationID
}

// NewMemoryRepository constructs a MemoryRepository.
func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		records: make(map[string]AttestationRecord),
		byRunID: make(map[string]string),
	}
}

func (r *MemoryRepository) Save(ctx context.Context, record AttestationRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.records[record.ID] = record
	r.byRunID[record.RunID] = record.ID
	return nil
}

func (r *MemoryRepository) GetByID(ctx context.Context, id string) (*AttestationRecord, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	rec, exists := r.records[id]
	if !exists {
		return nil, ErrAttestationNotFound
	}
	return &rec, nil
}

func (r *MemoryRepository) GetByRunID(ctx context.Context, runID string) (*AttestationRecord, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	id, exists := r.byRunID[runID]
	if !exists {
		return nil, ErrAttestationNotFound
	}
	rec := r.records[id]
	return &rec, nil
}
