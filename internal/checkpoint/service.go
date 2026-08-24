package checkpoint

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"

	"github.com/sentinelmesh/sentinelmesh/internal/events"
	"github.com/sentinelmesh/sentinelmesh/internal/observability"
	"github.com/sentinelmesh/sentinelmesh/internal/outbox"
	"github.com/sentinelmesh/sentinelmesh/internal/repository"
)

// SaveCheckpointRequest encapsulates input for checkpoint creation.
type SaveCheckpointRequest struct {
	RunID          string            `json:"run_id"`
	AgentID        string            `json:"agent_id"`
	TenantID       string            `json:"tenant_id"`
	SequenceNumber int64             `json:"sequence_number"`
	StateInline    json.RawMessage   `json:"state_inline,omitempty"`
	StateURI       string            `json:"state_uri,omitempty"`
	StateChecksum  string            `json:"state_checksum,omitempty"`
	SizeBytes      int64             `json:"size_bytes,omitempty"`
	Metadata       map[string]string `json:"metadata,omitempty"`
}

// Service coordinates checkpoint persistence, integrity validation, and outbox event dispatch.
type Service struct {
	repo       Repository
	outboxRepo outbox.Repository
	txManager  repository.TxManager
}

// NewService constructs a checkpoint Service.
func NewService(
	repo Repository,
	outboxRepo outbox.Repository,
	txManager repository.TxManager,
) *Service {
	return &Service{
		repo:       repo,
		outboxRepo: outboxRepo,
		txManager:  txManager,
	}
}

// SaveCheckpoint validates and persists an application checkpoint, publishing an outbox event.
func (s *Service) SaveCheckpoint(ctx context.Context, req SaveCheckpointRequest) (*Checkpoint, error) {
	ctx, span := observability.StartSpan(ctx, "checkpoint.save")
	defer span.End()

	m := observability.GetMetrics()
	startTime := time.Now()

	observability.InjectSpanAttributes(span, req.RunID, req.AgentID, req.TenantID, observability.GetCorrelationID(ctx), 0)

	if req.RunID == "" || req.AgentID == "" || req.TenantID == "" {
		err := fmt.Errorf("%w: missing required run, agent, or tenant identifier", ErrInvalidCheckpoint)
		observability.RecordSpanError(span, err)
		m.CheckpointFailuresTotal.WithLabelValues("validation_error").Inc()
		return nil, err
	}
	if req.SequenceNumber <= 0 {
		err := fmt.Errorf("%w: sequence number must be greater than 0", ErrInvalidCheckpoint)
		observability.RecordSpanError(span, err)
		m.CheckpointFailuresTotal.WithLabelValues("validation_error").Inc()
		return nil, err
	}
	if len(req.StateInline) == 0 && req.StateURI == "" {
		err := fmt.Errorf("%w: must provide either state_inline or state_uri", ErrInvalidCheckpoint)
		observability.RecordSpanError(span, err)
		m.CheckpointFailuresTotal.WithLabelValues("validation_error").Inc()
		return nil, err
	}

	// 1. Monotonicity validation against existing latest checkpoint
	latest, getErr := s.repo.GetLatest(ctx, req.RunID)
	if getErr != nil && !errors.Is(getErr, ErrCheckpointNotFound) {
		observability.RecordSpanError(span, getErr)
		m.CheckpointFailuresTotal.WithLabelValues("storage_error").Inc()
		return nil, fmt.Errorf("failed to check latest checkpoint: %w", getErr)
	}
	if getErr == nil && latest != nil {
		if req.SequenceNumber < latest.SequenceNumber {
			err := fmt.Errorf("%w: requested sequence %d is less than latest %d",
				ErrNonMonotonicSeq, req.SequenceNumber, latest.SequenceNumber)
			observability.RecordSpanError(span, err)
			m.CheckpointFailuresTotal.WithLabelValues("sequence_conflict").Inc()
			return nil, err
		}
	}

	// 2. Check for idempotent retry on same sequence number
	checksum := req.StateChecksum
	sizeBytes := req.SizeBytes
	tier := "inline"
	if len(req.StateInline) > 0 {
		checksum = ComputeCanonicalChecksum(req.StateInline)
		sizeBytes = int64(len(req.StateInline))
	} else if req.StateURI != "" {
		tier = "remote"
	}

	existing, existErr := s.repo.GetBySequence(ctx, req.RunID, req.SequenceNumber)
	if existErr == nil && existing != nil {
		if existing.StateChecksum == checksum {
			return existing, nil // Idempotent success
		}
		err := fmt.Errorf("%w: sequence %d already exists with different checksum",
			ErrSequenceConflict, req.SequenceNumber)
		observability.RecordSpanError(span, err)
		m.CheckpointFailuresTotal.WithLabelValues("sequence_conflict").Inc()
		return nil, err
	}

	cp := Checkpoint{
		ID:             uuid.NewString(),
		RunID:          req.RunID,
		AgentID:        req.AgentID,
		TenantID:       req.TenantID,
		SequenceNumber: req.SequenceNumber,
		StateInline:    req.StateInline,
		StateURI:       req.StateURI,
		StateChecksum:  checksum,
		SizeBytes:      sizeBytes,
		Metadata:       req.Metadata,
		CreatedAt:      time.Now(),
	}

	if err := cp.Validate(); err != nil {
		observability.RecordSpanError(span, err)
		m.CheckpointFailuresTotal.WithLabelValues("validation_error").Inc()
		return nil, err
	}

	// 3. Persist checkpoint & outbox event
	var err error
	if s.txManager != nil {
		err = s.txManager.WithinTx(ctx, func(txCtx context.Context) error {
			if err := s.repo.Save(txCtx, cp); err != nil {
				return err
			}
			return s.recordOutboxEvent(txCtx, cp)
		})
	} else {
		if err := s.repo.Save(ctx, cp); err != nil {
			observability.RecordSpanError(span, err)
			m.CheckpointFailuresTotal.WithLabelValues("storage_error").Inc()
			return nil, err
		}
		_ = s.recordOutboxEvent(ctx, cp)
	}

	if err != nil {
		observability.RecordSpanError(span, err)
		m.CheckpointFailuresTotal.WithLabelValues("storage_error").Inc()
		return nil, err
	}

	duration := time.Since(startTime).Seconds()
	m.CheckpointSavedTotal.WithLabelValues(tier).Inc()
	m.CheckpointDurationSec.WithLabelValues(tier).Observe(duration)
	m.CheckpointSizeBytes.WithLabelValues(tier).Observe(float64(cp.SizeBytes))

	span.SetAttributes(
		attribute.String("checkpoint.id", cp.ID),
		attribute.Int64("checkpoint.sequence_number", cp.SequenceNumber),
		attribute.Int64("checkpoint.size_bytes", cp.SizeBytes),
		attribute.String("checkpoint.storage_tier", tier),
	)

	return &cp, nil
}

func (s *Service) recordOutboxEvent(ctx context.Context, cp Checkpoint) error {
	if s.outboxRepo == nil {
		return nil
	}

	traceID, _ := observability.GetTraceAndSpanID(ctx)
	corrID := observability.GetCorrelationID(ctx)

	payload, err := json.Marshal(events.CheckpointSavedPayload{
		CheckpointID:   cp.ID,
		RunID:          cp.RunID,
		AgentID:        cp.AgentID,
		TenantID:       cp.TenantID,
		SequenceNumber: cp.SequenceNumber,
		StateChecksum:  cp.StateChecksum,
		SizeBytes:      cp.SizeBytes,
		CreatedAt:      cp.CreatedAt,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal checkpoint payload: %w", err)
	}

	evt := events.Event{
		EventID:       uuid.NewString(),
		EventType:     events.SubjectCheckpointSaved,
		SchemaVersion: 1,
		AggregateType: "Checkpoint",
		AggregateID:   cp.ID,
		TenantID:      cp.TenantID,
		CorrelationID: corrID,
		TraceParent:   traceID,
		OccurredAt:    time.Now(),
		Payload:       payload,
	}

	return s.outboxRepo.Insert(ctx, evt)
}

// GetLatestCheckpoint retrieves the latest valid checkpoint for a run.
func (s *Service) GetLatestCheckpoint(ctx context.Context, runID string) (*Checkpoint, error) {
	ctx, span := observability.StartSpan(ctx, "checkpoint.get_latest")
	defer span.End()

	if runID == "" {
		return nil, fmt.Errorf("empty runID")
	}
	cp, err := s.repo.GetLatest(ctx, runID)
	if err != nil {
		observability.RecordSpanError(span, err)
		return nil, err
	}
	if !cp.VerifyIntegrity() {
		err := ErrCorruptedCheckpoint
		observability.RecordSpanError(span, err)
		return nil, err
	}
	return cp, nil
}

// GetBySequence retrieves a checkpoint by exact sequence number.
func (s *Service) GetBySequence(ctx context.Context, runID string, seq int64) (*Checkpoint, error) {
	ctx, span := observability.StartSpan(ctx, "checkpoint.get_by_sequence")
	defer span.End()

	cp, err := s.repo.GetBySequence(ctx, runID, seq)
	if err != nil {
		observability.RecordSpanError(span, err)
		return nil, err
	}
	if !cp.VerifyIntegrity() {
		err := ErrCorruptedCheckpoint
		observability.RecordSpanError(span, err)
		return nil, err
	}
	return cp, nil
}

// ListCheckpoints returns all checkpoints for a run.
func (s *Service) ListCheckpoints(ctx context.Context, runID string) ([]Checkpoint, error) {
	ctx, span := observability.StartSpan(ctx, "checkpoint.list")
	defer span.End()

	return s.repo.List(ctx, runID)
}
