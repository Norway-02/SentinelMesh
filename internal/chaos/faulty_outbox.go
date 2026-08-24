package chaos

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/sentinelmesh/sentinelmesh/internal/events"
	"github.com/sentinelmesh/sentinelmesh/internal/outbox"
)

// FaultyOutboxRepository wraps an outbox.Repository to simulate messaging failures.
type FaultyOutboxRepository struct {
	mu          sync.RWMutex
	underlying  outbox.Repository
	controller  *FaultController
	events      []events.Event
	droppedList []events.Event
}

// NewFaultyOutboxRepository constructs a FaultyOutboxRepository wrapper.
func NewFaultyOutboxRepository(underlying outbox.Repository, controller *FaultController) *FaultyOutboxRepository {
	return &FaultyOutboxRepository{
		underlying: underlying,
		controller: controller,
		events:     make([]events.Event, 0),
	}
}

// Insert adds an event to the outbox unless intercepted by a fault.
func (r *FaultyOutboxRepository) Insert(ctx context.Context, event events.Event) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if trigger, spec := r.controller.ShouldTrigger("Insert"); trigger {
		if spec.Delay > 0 {
			time.Sleep(spec.Delay)
		}
		if spec.Type == FaultDrop {
			r.droppedList = append(r.droppedList, event)
			return nil // Silently dropped
		}
		if spec.Type == FaultError {
			errMsg := spec.ErrorMsg
			if errMsg == "" {
				errMsg = "simulated outbox insert error"
			}
			return errors.New(errMsg)
		}
		if spec.Type == FaultDuplicate {
			// Insert original and a duplicate
			if err := r.underlying.Insert(ctx, event); err != nil {
				return err
			}
			dupEvent := event
			dupEvent.EventID = uuid.NewString()
			r.events = append(r.events, event, dupEvent)
			return r.underlying.Insert(ctx, dupEvent)
		}
	}

	r.events = append(r.events, event)
	return r.underlying.Insert(ctx, event)
}

// Claim claims events for publishing.
func (r *FaultyOutboxRepository) Claim(ctx context.Context, batchSize int, ownerID string, claimDuration time.Duration) ([]events.Event, error) {
	if trigger, spec := r.controller.ShouldTrigger("Claim"); trigger {
		if spec.Delay > 0 {
			time.Sleep(spec.Delay)
		}
		if spec.Type == FaultError {
			return nil, errors.New("simulated outbox claim failure")
		}
	}
	return r.underlying.Claim(ctx, batchSize, ownerID, claimDuration)
}

// MarkPublished marks an event as published.
func (r *FaultyOutboxRepository) MarkPublished(ctx context.Context, eventID string) error {
	if trigger, spec := r.controller.ShouldTrigger("MarkPublished"); trigger {
		if spec.Type == FaultError {
			return errors.New("simulated publish acknowledge failure")
		}
	}
	return r.underlying.MarkPublished(ctx, eventID)
}

// MarkFailed marks an event as failed.
func (r *FaultyOutboxRepository) MarkFailed(ctx context.Context, eventID string, errStr string) error {
	return r.underlying.MarkFailed(ctx, eventID, errStr)
}

// GetRecordedEvents returns all events processed through this outbox.
func (r *FaultyOutboxRepository) GetRecordedEvents() []events.Event {
	r.mu.RLock()
	defer r.mu.RUnlock()
	res := make([]events.Event, len(r.events))
	copy(res, r.events)
	return res
}

// GetDroppedEvents returns all events that were dropped by FaultDrop.
func (r *FaultyOutboxRepository) GetDroppedEvents() []events.Event {
	r.mu.RLock()
	defer r.mu.RUnlock()
	res := make([]events.Event, len(r.droppedList))
	copy(res, r.droppedList)
	return res
}
