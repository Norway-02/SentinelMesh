package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"

	"github.com/sentinelmesh/sentinelmesh/internal/checkpoint"
	"github.com/sentinelmesh/sentinelmesh/internal/domain"
	"github.com/sentinelmesh/sentinelmesh/internal/events"
	"github.com/sentinelmesh/sentinelmesh/internal/observability"
	"github.com/sentinelmesh/sentinelmesh/internal/outbox"
	"github.com/sentinelmesh/sentinelmesh/internal/repository"
	"github.com/sentinelmesh/sentinelmesh/internal/scheduler"
	"github.com/sentinelmesh/sentinelmesh/internal/types"
)

// RecoveryCoordinator orchestrates self-healing failover and checkpoint restoration.
type RecoveryCoordinator struct {
	runRepo         repository.RunRepository
	checkpointSvc   *checkpoint.Service
	schedulerSvc    *scheduler.Service
	outboxRepo      outbox.Repository
	txManager       repository.TxManager
}

// NewRecoveryCoordinator constructs a RecoveryCoordinator instance.
func NewRecoveryCoordinator(
	runRepo repository.RunRepository,
	checkpointSvc *checkpoint.Service,
	schedulerSvc *scheduler.Service,
	outboxRepo outbox.Repository,
	txManager repository.TxManager,
) *RecoveryCoordinator {
	return &RecoveryCoordinator{
		runRepo:       runRepo,
		checkpointSvc: checkpointSvc,
		schedulerSvc:  schedulerSvc,
		outboxRepo:    outboxRepo,
		txManager:     txManager,
	}
}

// HandleRecovery processes a recovery request for an interrupted run.
// HandleRecovery processes a recovery request for an interrupted run.
func (c *RecoveryCoordinator) HandleRecovery(ctx context.Context, req events.RunRecoveryRequestedPayload) error {
	ctx, span := observability.StartSpan(ctx, "recovery.handle")
	defer span.End()

	m := observability.GetMetrics()
	startTime := time.Now()

	observability.InjectSpanAttributes(span, req.RunID, req.AgentID, req.TenantID, observability.GetCorrelationID(ctx), req.RecoveryGeneration)
	span.SetAttributes(
		attribute.String("recovery.failed_node_id", req.FailedNodeID),
		attribute.Int("recovery.generation", req.RecoveryGeneration),
	)

	m.RecoveryRequestsTotal.WithLabelValues("node_failure").Inc()

	run, err := c.runRepo.Get(ctx, req.RunID)
	if err != nil {
		observability.RecordSpanError(span, err)
		m.RecoveryFailureTotal.WithLabelValues("run_not_found").Inc()
		m.RecoveryDurationSec.WithLabelValues("failure").Observe(time.Since(startTime).Seconds())
		return fmt.Errorf("failed to fetch run %s: %w", req.RunID, err)
	}

	// Invariant 5: Stale recovery prevention (generation monotonicity)
	if req.RecoveryGeneration < run.RecoveryGeneration {
		span.SetAttributes(attribute.String("recovery.result", "stale_ignored"))
		return nil // Stale recovery request, ignore
	}

	// 1. Transition run to RECOVERING
	_ = run.TransitionTo(types.StateRecovering)
	run.RecoveryGeneration = req.RecoveryGeneration
	if err := c.runRepo.Update(ctx, run); err != nil {
		observability.RecordSpanError(span, err)
		m.RecoveryFailureTotal.WithLabelValues("state_transition_failed").Inc()
		m.RecoveryDurationSec.WithLabelValues("failure").Observe(time.Since(startTime).Seconds())
		return fmt.Errorf("failed to transition run to RECOVERING: %w", err)
	}
	// Fetch updated run with new Version
	run, _ = c.runRepo.Get(ctx, req.RunID)

	// 2. Fetch latest valid checkpoint
	var cpPayload *events.CheckpointMetadataPayload
	var cpCreatedAt time.Time
	if c.checkpointSvc != nil {
		cp, err := c.checkpointSvc.GetLatestCheckpoint(ctx, req.RunID)
		if err != nil {
			if errors.Is(err, checkpoint.ErrCorruptedCheckpoint) {
				// Invariant 1: Corrupted checkpoint must NEVER be restored
				m.RecoveryFailureTotal.WithLabelValues("corrupted_checkpoint").Inc()
				m.RecoveryDurationSec.WithLabelValues("failure").Observe(time.Since(startTime).Seconds())
				return c.failRecovery(ctx, &run, "corrupted checkpoint state integrity failure")
			}
			if !errors.Is(err, checkpoint.ErrCheckpointNotFound) {
				observability.RecordSpanError(span, err)
				m.RecoveryFailureTotal.WithLabelValues("checkpoint_fetch_error").Inc()
				m.RecoveryDurationSec.WithLabelValues("failure").Observe(time.Since(startTime).Seconds())
				return fmt.Errorf("failed to retrieve checkpoint: %w", err)
			}
		} else if cp != nil {
			cpCreatedAt = cp.CreatedAt
			cpPayload = &events.CheckpointMetadataPayload{
				ID:        cp.ID,
				Sequence:  cp.SequenceNumber,
				Checksum:  cp.StateChecksum,
				SizeBytes: cp.SizeBytes,
			}
		}
	}

	// 3. Delegate deterministic re-placement to Scheduler (excluding failed cluster & failed node)
	var excludeClusters []string
	if req.FailedClusterID != "" {
		excludeClusters = append(excludeClusters, req.FailedClusterID)
	}
	var excludeNodes []string
	if req.FailedNodeID != "" {
		excludeNodes = append(excludeNodes, req.FailedNodeID)
	}

	reschedReq := scheduler.RescheduleRequest{
		RunID:              req.RunID,
		ExcludeClusterIDs:  excludeClusters,
		ExcludeNodeIDs:     excludeNodes,
		RecoveryGeneration: req.RecoveryGeneration,
		Checkpoint:         cpPayload,
	}

	if err := c.schedulerSvc.RescheduleRun(ctx, reschedReq); err != nil {
		m.RecoveryFailureTotal.WithLabelValues("reschedule_failed").Inc()
		m.RecoveryDurationSec.WithLabelValues("failure").Observe(time.Since(startTime).Seconds())
		return c.failRecovery(ctx, &run, fmt.Sprintf("rescheduling failed: %v", err))
	}

	// 4. Emit RunRecovered event
	updatedRun, _ := c.runRepo.Get(ctx, req.RunID)
	now := time.Now()

	var restoredCPID string
	var restoredSeq int64
	if cpPayload != nil {
		restoredCPID = cpPayload.ID
		restoredSeq = cpPayload.Sequence
	}

	traceID, _ := observability.GetTraceAndSpanID(ctx)
	corrID := observability.GetCorrelationID(ctx)

	if c.outboxRepo != nil {
		recPayload, _ := json.Marshal(events.RunRecoveredPayload{
			RunID:                req.RunID,
			AgentID:              req.AgentID,
			TenantID:             req.TenantID,
			TargetClusterID:      updatedRun.Cluster,
			TargetNodeID:         updatedRun.Node,
			RecoveryGeneration:   req.RecoveryGeneration,
			FencingToken:         updatedRun.FencingToken,
			RestoredCheckpointID: restoredCPID,
			RestoredSequence:     restoredSeq,
			RecoveredAt:          now,
		})

		_ = c.outboxRepo.Insert(ctx, events.Event{
			EventID:       uuid.NewString(),
			EventType:     events.SubjectRunRecovered,
			SchemaVersion: 1,
			AggregateType: "Run",
			AggregateID:   req.RunID,
			TenantID:      req.TenantID,
			CorrelationID: corrID,
			TraceParent:   traceID,
			OccurredAt:    now,
			Payload:       recPayload,
		})
	}

	// Record success metrics & thesis effectiveness metrics
	duration := time.Since(startTime).Seconds()
	targetCluster := updatedRun.Cluster
	if targetCluster == "" {
		targetCluster = "default"
	}
	m.RecoverySuccessTotal.WithLabelValues(targetCluster).Inc()
	m.RecoveryGenerationTotal.WithLabelValues(fmt.Sprintf("gen_%d", req.RecoveryGeneration)).Inc()
	m.RecoveryDurationSec.WithLabelValues("success").Observe(duration)

	if restoredSeq > 0 {
		m.RecoveryPointSteps.WithLabelValues("agent").Observe(float64(restoredSeq))
	}
	if !cpCreatedAt.IsZero() {
		m.RecoveryCheckpointAge.WithLabelValues("agent").Observe(time.Since(cpCreatedAt).Seconds())
	}
	// Lost work steps: difference between run sequence and restored sequence
	var lostSteps int64 = 0
	if req.SequenceNumber > restoredSeq {
		lostSteps = req.SequenceNumber - restoredSeq
	}
	m.RecoveryLostWorkSteps.WithLabelValues("agent").Observe(float64(lostSteps))

	span.SetAttributes(
		attribute.String("recovery.target_node_id", updatedRun.Node),
		attribute.Int64("recovery.restored_sequence", restoredSeq),
		attribute.Int64("recovery.lost_work_steps", lostSteps),
		attribute.Float64("recovery.duration_seconds", duration),
	)

	return nil
}

func (c *RecoveryCoordinator) failRecovery(ctx context.Context, run *domain.AgentRun, reason string) error {
	currentRun, err := c.runRepo.Get(ctx, run.ID)
	if err == nil {
		run = &currentRun
	}
	run.State = types.StateFailed
	run.FailureReason = fmt.Sprintf("Recovery Failed: %s", reason)
	_ = c.runRepo.Update(ctx, *run)

	traceID, _ := observability.GetTraceAndSpanID(ctx)
	corrID := observability.GetCorrelationID(ctx)

	if c.outboxRepo != nil {
		failPayload, _ := json.Marshal(events.RunRecoveryFailedPayload{
			RunID:              run.ID,
			AgentID:            run.AgentID,
			TenantID:           run.TenantID,
			RecoveryGeneration: run.RecoveryGeneration,
			Reason:             reason,
			FailedAt:           time.Now(),
		})

		_ = c.outboxRepo.Insert(ctx, events.Event{
			EventID:       uuid.NewString(),
			EventType:     events.SubjectRunRecoveryFailed,
			SchemaVersion: 1,
			AggregateType: "Run",
			AggregateID:   run.ID,
			TenantID:      run.TenantID,
			CorrelationID: corrID,
			TraceParent:   traceID,
			OccurredAt:    time.Now(),
			Payload:       failPayload,
		})
	}

	return fmt.Errorf("recovery failed: %s", reason)
}
