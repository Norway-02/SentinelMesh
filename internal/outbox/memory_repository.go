package outbox

import (
	"context"
	"sync"
	"time"

	"github.com/sentinelmesh/sentinelmesh/internal/events"
)

type MemoryRepository struct {
	mu     sync.RWMutex
	events []events.Event
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		events: make([]events.Event, 0),
	}
}

func (r *MemoryRepository) Insert(ctx context.Context, event events.Event) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, event)
	return nil
}

func (r *MemoryRepository) Claim(ctx context.Context, batchSize int, ownerID string, claimDuration time.Duration) ([]events.Event, error) {
	return nil, nil // No-op for memory
}

func (r *MemoryRepository) MarkPublished(ctx context.Context, eventID string) error {
	return nil
}

func (r *MemoryRepository) MarkFailed(ctx context.Context, eventID string, errStr string) error {
	return nil
}

func (r *MemoryRepository) GetEvents() []events.Event {
	r.mu.RLock()
	defer r.mu.RUnlock()
	res := make([]events.Event, len(r.events))
	copy(res, r.events)
	return res
}

