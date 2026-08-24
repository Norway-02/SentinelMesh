package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/sentinelmesh/sentinelmesh/internal/events"
	"github.com/sentinelmesh/sentinelmesh/internal/observability"
	"github.com/sentinelmesh/sentinelmesh/internal/outbox"
	"github.com/sentinelmesh/sentinelmesh/internal/repository"
	"github.com/sentinelmesh/sentinelmesh/internal/types"
)

// SyncService synchronizes runtime execution states with persistent domain state
// in PostgreSQL and emits RunStateChanged events atomically via the Transactional Outbox.
type SyncService struct {
	txManager repository.TxManager
	runRepo   repository.RunRepository
	outbox    outbox.Repository
}

// NewSyncService constructs a SyncService.
func NewSyncService(
	txManager repository.TxManager,
	runRepo repository.RunRepository,
	outboxRepo outbox.Repository,
) *SyncService {
	return &SyncService{
		txManager: txManager,
		runRepo:   runRepo,
		outbox:    outboxRepo,
	}
}

// HandleStateChange is a StateChangeCallback suitable for registering with the Supervisor.
func (s *SyncService) HandleStateChange(status ExecutionStatus) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.SyncStatus(ctx, status); err != nil {
		slog.Error("Failed to sync runtime state to database",
			"run_id", status.RunID,
			"target_state", status.State,
			"error", err,
		)
	}
}

// SyncStatus updates the domain run entity and enqueues the outbox event atomically.
func (s *SyncService) SyncStatus(ctx context.Context, status ExecutionStatus) error {
	ctx, span := observability.StartSpan(ctx, "runtime.sync_status")
	defer span.End()

	return s.txManager.WithinTx(ctx, func(txCtx context.Context) error {
		run, err := s.runRepo.Get(txCtx, status.RunID)
		if err != nil {
			observability.RecordSpanError(span, err)
			return fmt.Errorf("failed to fetch run %s: %w", status.RunID, err)
		}

		observability.InjectSpanAttributes(span, run.ID, run.AgentID, run.TenantID, observability.GetCorrelationID(txCtx), run.RecoveryGeneration)

		oldState := run.State
		if oldState == status.State {
			// No transition needed
			return nil
		}

		// Apply state machine transition
		if err := run.TransitionTo(status.State); err != nil {
			// If direct transition is invalid (e.g. from SCHEDULED directly to RUNNING),
			// drive through intermediate STARTING state if valid
			if oldState == types.StateScheduled && status.State == types.StateRunning {
				_ = run.TransitionTo(types.StateStarting)
				_ = run.TransitionTo(types.StateRunning)
			} else {
				observability.RecordSpanError(span, err)
				return fmt.Errorf("invalid state transition from %s to %s for run %s: %w",
					oldState, status.State, status.RunID, err)
			}
		}

		if status.StartedAt != nil && run.StartedAt == nil {
			run.StartedAt = status.StartedAt
		}
		if status.FinishedAt != nil {
			run.FinishedAt = status.FinishedAt
		}
		if status.ErrorReason != "" {
			run.FailureReason = status.ErrorReason
		}

		// Persist updated run state with OCC
		if err := s.runRepo.Update(txCtx, run); err != nil {
			observability.RecordSpanError(span, err)
			return fmt.Errorf("failed to update run %s in repository: %w", run.ID, err)
		}

		// Create Outbox event
		payload := events.RunStateChangedPayload{
			RunID:     run.ID,
			AgentID:   run.AgentID,
			FromState: string(oldState),
			ToState:   string(run.State),
			Version:   run.Version,
		}

		payloadBytes, err := json.Marshal(payload)
		if err != nil {
			observability.RecordSpanError(span, err)
			return fmt.Errorf("failed to marshal RunStateChangedPayload: %w", err)
		}

		traceID, _ := observability.GetTraceAndSpanID(txCtx)
		corrID := observability.GetCorrelationID(txCtx)
		if corrID == "" {
			corrID = run.ID
		}

		evt := events.Event{
			EventID:       uuid.NewString(),
			EventType:     events.SubjectRunStateChanged,
			SchemaVersion: 1,
			AggregateType: "run",
			AggregateID:   run.ID,
			TenantID:      run.TenantID,
			CorrelationID: corrID,
			TraceParent:   traceID,
			OccurredAt:    time.Now(),
			Payload:       payloadBytes,
		}

		if err := s.outbox.Insert(txCtx, evt); err != nil {
			observability.RecordSpanError(span, err)
			return fmt.Errorf("failed to save outbox event for run %s: %w", run.ID, err)
		}

		slog.InfoContext(txCtx, "Successfully synchronized run state",
			slog.String("run_id", run.ID),
			slog.String("from_state", string(oldState)),
			slog.String("to_state", string(run.State)),
			slog.Int64("version", run.Version),
		)

		return nil
	})
}
