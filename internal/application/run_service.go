package application

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/sentinelmesh/sentinelmesh/internal/domain"
	"github.com/sentinelmesh/sentinelmesh/internal/events"
	"github.com/sentinelmesh/sentinelmesh/internal/observability"
	"github.com/sentinelmesh/sentinelmesh/internal/outbox"
	"github.com/sentinelmesh/sentinelmesh/internal/repository"
	"github.com/sentinelmesh/sentinelmesh/internal/types"
)

type RunService struct {
	txManager repository.TxManager
	agentRepo repository.AgentRepository
	runRepo   repository.RunRepository
	outbox    outbox.Repository
}

func NewRunService(
	txManager repository.TxManager,
	agentRepo repository.AgentRepository,
	runRepo repository.RunRepository,
	outboxRepo outbox.Repository,
) *RunService {
	return &RunService{
		txManager: txManager,
		agentRepo: agentRepo,
		runRepo:   runRepo,
		outbox:    outboxRepo,
	}
}

func (s *RunService) CreateRun(ctx context.Context, agentID string) (domain.AgentRun, error) {
	ctx, span := observability.StartSpan(ctx, "application.create_run")
	defer span.End()

	m := observability.GetMetrics()

	agent, err := s.agentRepo.Get(ctx, agentID)
	if err != nil {
		observability.RecordSpanError(span, err)
		return domain.AgentRun{}, fmt.Errorf("failed to get agent: %w", err)
	}

	now := time.Now()
	run := domain.AgentRun{
		ID:        uuid.NewString(),
		AgentID:   agentID,
		TenantID:  agent.TenantID,
		State:     types.StateCreated,
		StartedAt: &now,
		Version:   1,
	}

	observability.InjectSpanAttributes(span, run.ID, run.AgentID, run.TenantID, observability.GetCorrelationID(ctx), 0)

	if err := run.Validate(); err != nil {
		observability.RecordSpanError(span, err)
		return domain.AgentRun{}, fmt.Errorf("invalid run: %w", err)
	}

	traceID, _ := observability.GetTraceAndSpanID(ctx)
	corrID := observability.GetCorrelationID(ctx)

	err = s.txManager.WithinTx(ctx, func(txCtx context.Context) error {
		if err := s.runRepo.Create(txCtx, run); err != nil {
			return err
		}

		payload, _ := json.Marshal(events.RunCreatedPayload{
			RunID:     run.ID,
			AgentID:   run.AgentID,
			CreatedAt: *run.StartedAt,
		})

		evt := events.Event{
			EventID:       uuid.NewString(),
			EventType:     events.SubjectRunCreated,
			SchemaVersion: 1,
			AggregateType: "Run",
			AggregateID:   run.ID,
			TenantID:      run.TenantID,
			CorrelationID: corrID,
			TraceParent:   traceID,
			OccurredAt:    time.Now(),
			Payload:       payload,
		}

		return s.outbox.Insert(txCtx, evt)
	})

	if err != nil {
		observability.RecordSpanError(span, err)
		return domain.AgentRun{}, err
	}

	m.RunsCreatedTotal.WithLabelValues("standard").Inc()
	m.ActiveRuns.WithLabelValues(string(types.StateCreated)).Inc()

	return run, nil
}

func (s *RunService) GetRun(ctx context.Context, id string) (domain.AgentRun, error) {
	if id == "" {
		return domain.AgentRun{}, fmt.Errorf("invalid run id: empty")
	}
	return s.runRepo.Get(ctx, id)
}

func (s *RunService) CancelRun(ctx context.Context, id string) (domain.AgentRun, error) {
	var run domain.AgentRun
	
	err := s.txManager.WithinTx(ctx, func(txCtx context.Context) error {
		var err error
		run, err = s.runRepo.Get(txCtx, id)
		if err != nil {
			return err
		}

		fromState := run.State

		if err := run.TransitionTo(types.StateCancelled); err != nil {
			return fmt.Errorf("transition failed: %w", err)
		}

		if err := s.runRepo.Update(txCtx, run); err != nil {
			return err
		}

		payload, _ := json.Marshal(events.RunStateChangedPayload{
			RunID:     run.ID,
			AgentID:   run.AgentID,
			FromState: string(fromState),
			ToState:   string(run.State),
			Version:   run.Version,
		})

		evt := events.Event{
			EventID:       uuid.NewString(),
			EventType:     events.SubjectRunStateChanged,
			SchemaVersion: 1,
			AggregateType: "Run",
			AggregateID:   run.ID,
			TenantID:      run.TenantID,
			OccurredAt:    time.Now(),
			Payload:       payload,
		}

		return s.outbox.Insert(txCtx, evt)
	})

	if err != nil {
		return domain.AgentRun{}, err
	}

	return run, nil
}

func (s *RunService) GetRunState(ctx context.Context, id string) (types.AgentState, error) {
	run, err := s.runRepo.Get(ctx, id)
	if err != nil {
		return "", err
	}
	return run.State, nil
}
