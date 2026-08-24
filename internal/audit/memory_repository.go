package audit

import (
	"context"
	"sync"
)

// MemoryRepository provides a thread-safe in-memory implementation of the audit Repository.
type MemoryRepository struct {
	mu     sync.RWMutex
	events []AuditEvent
}

// NewMemoryRepository constructs a MemoryRepository.
func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		events: make([]AuditEvent, 0),
	}
}

// Insert saves an audit event.
func (r *MemoryRepository) Insert(ctx context.Context, event AuditEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, event)
	return nil
}

// GetByRunID returns all audit events for a given runID.
func (r *MemoryRepository) GetByRunID(ctx context.Context, runID string) ([]AuditEvent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []AuditEvent
	for _, e := range r.events {
		if e.RunID == runID {
			result = append(result, e)
		}
	}
	return result, nil
}

// List queries audit events with optional filtering.
func (r *MemoryRepository) List(ctx context.Context, filter AuditFilter) ([]AuditEvent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []AuditEvent
	for _, e := range r.events {
		if filter.RunID != "" && e.RunID != filter.RunID {
			continue
		}
		if filter.AgentID != "" && e.AgentID != filter.AgentID {
			continue
		}
		if filter.TenantID != "" && e.TenantID != filter.TenantID {
			continue
		}
		if filter.Decision != "" && e.Decision != filter.Decision {
			continue
		}
		if filter.Severity != "" && e.Severity != filter.Severity {
			continue
		}
		result = append(result, e)
		if filter.Limit > 0 && len(result) >= filter.Limit {
			break
		}
	}
	return result, nil
}

// GetEvents returns a copy of all events in the repository.
func (r *MemoryRepository) GetEvents() []AuditEvent {
	r.mu.RLock()
	defer r.mu.RUnlock()
	res := make([]AuditEvent, len(r.events))
	copy(res, r.events)
	return res
}
