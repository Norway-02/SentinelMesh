package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"

	"github.com/sentinelmesh/sentinelmesh/internal/domain"
	"github.com/sentinelmesh/sentinelmesh/internal/events"
	"github.com/sentinelmesh/sentinelmesh/internal/observability"
	"github.com/sentinelmesh/sentinelmesh/internal/outbox"
	"github.com/sentinelmesh/sentinelmesh/internal/repository"
	"github.com/sentinelmesh/sentinelmesh/internal/types"
)

// Service encapsulates the scheduling orchestration logic
type Service struct {
	txManager           repository.TxManager
	agentRepo           repository.AgentRepository
	runRepo             repository.RunRepository
	assignmentRepo      repository.AssignmentRepository
	outboxRepo          outbox.Repository
	resourceProv        ResourceProvider
	clusterResourceProv ClusterResourceProvider
	scoringPolicy       ClusterScoringPolicy
}

func NewService(
	txManager repository.TxManager,
	agentRepo repository.AgentRepository,
	runRepo repository.RunRepository,
	assignmentRepo repository.AssignmentRepository,
	outboxRepo outbox.Repository,
	resourceProv ResourceProvider,
) *Service {
	return &Service{
		txManager:      txManager,
		agentRepo:      agentRepo,
		runRepo:        runRepo,
		assignmentRepo: assignmentRepo,
		outboxRepo:     outboxRepo,
		resourceProv:   resourceProv,
		scoringPolicy:  DefaultClusterScoringPolicy(),
	}
}

// WithClusterResourceProvider attaches multi-cluster provider and custom cluster scoring policy.
func (s *Service) WithClusterResourceProvider(prov ClusterResourceProvider, policy ClusterScoringPolicy) *Service {
	s.clusterResourceProv = prov
	s.scoringPolicy = policy
	return s
}

// ScheduleRun processes a RunCreated event and attempts to schedule it.
func (s *Service) ScheduleRun(ctx context.Context, runID string) error {
	ctx, span := observability.StartSpan(ctx, "scheduler.decision")
	defer span.End()

	m := observability.GetMetrics()
	startTime := time.Now()

	logger := slog.With("run_id", runID)

	run, err := s.runRepo.Get(ctx, runID)
	if err != nil {
		observability.RecordSpanError(span, err)
		return fmt.Errorf("failed to get run: %w", err)
	}

	observability.InjectSpanAttributes(span, run.ID, run.AgentID, run.TenantID, observability.GetCorrelationID(ctx), run.RecoveryGeneration)

	if string(run.State) != string(types.StateQueued) && string(run.State) != string(types.StateCreated) {
		logger.WarnContext(ctx, "Run is not in a schedulable state, skipping", slog.String("state", string(run.State)))
		return nil // skip processing
	}

	agent, err := s.agentRepo.Get(ctx, run.AgentID)
	if err != nil {
		observability.RecordSpanError(span, err)
		return fmt.Errorf("failed to get agent: %w", err)
	}

	var bestNode domain.Node
	var decision domain.SchedulingDecision
	var scoreErr error

	// Tier 1 & Tier 2: Check if MultiCluster provider is active
	if s.clusterResourceProv != nil {
		clusters, err := s.clusterResourceProv.ListClusters(ctx)
		if err != nil {
			observability.RecordSpanError(span, err)
			return fmt.Errorf("failed to list clusters: %w", err)
		}

		bestCluster, clusterDec, clusterErr := SelectBestCluster(&agent, clusters, nil, s.scoringPolicy)
		if clusterErr != nil {
			scoreErr = clusterErr
		} else {
			nodes, nodeErr := s.clusterResourceProv.ListNodes(ctx, bestCluster.ID)
			if nodeErr != nil {
				scoreErr = nodeErr
			} else {
				validNodes := filterNodes(&agent, nodes)
				bestNode, decision, scoreErr = scoreNodes(&agent, validNodes)
				decision.CandidatesConsidered = len(nodes)
				decision.CandidatesRejected = len(nodes) - len(validNodes)
				if bestNode.ClusterID == "" {
					bestNode.ClusterID = bestCluster.ID
				}
				span.SetAttributes(
					attribute.String("scheduler.tier1_selected_cluster", bestCluster.ID),
					attribute.Float64("scheduler.tier1_cluster_score", clusterDec.FinalScore),
				)
			}
		}
	} else {
		// Single cluster fallback
		nodes, err := s.resourceProv.ListNodes(ctx)
		if err != nil {
			observability.RecordSpanError(span, err)
			return fmt.Errorf("failed to list nodes: %w", err)
		}

		validNodes := filterNodes(&agent, nodes)
		bestNode, decision, scoreErr = scoreNodes(&agent, validNodes)
		decision.CandidatesConsidered = len(nodes)
		decision.CandidatesRejected = len(nodes) - len(validNodes)
	}

	m.SchedulerCandidatesTotal.WithLabelValues("fit").Add(float64(decision.CandidatesConsidered - decision.CandidatesRejected))
	if decision.CandidatesRejected > 0 {
		m.SchedulerRejectionsTotal.WithLabelValues("filter_mismatch").Add(float64(decision.CandidatesRejected))
	}

	duration := time.Since(startTime).Seconds()
	m.SchedulerDecisionDurationSec.WithLabelValues("deterministic-v1").Observe(duration)

	if scoreErr != nil {
		// Structural failure - no nodes/clusters fit.
		logger.WarnContext(ctx, "Scheduling failed", slog.String("reason", scoreErr.Error()))
		m.SchedulerDecisionsTotal.WithLabelValues("FAILURE", "deterministic-v1").Inc()
		observability.RecordSpanError(span, scoreErr)
		return s.failScheduling(ctx, &run, &agent, scoreErr.Error(), false)
	}

	m.SchedulerDecisionsTotal.WithLabelValues("SUCCESS", "deterministic-v1").Inc()
	span.SetAttributes(
		attribute.String("scheduler.selected_cluster_id", bestNode.ClusterID),
		attribute.String("scheduler.selected_node_id", bestNode.ID),
		attribute.Float64("scheduler.final_score", bestScore(decision)),
		attribute.Int("scheduler.candidates_considered", decision.CandidatesConsidered),
		attribute.Int("scheduler.candidates_rejected", decision.CandidatesRejected),
	)

	logger.InfoContext(ctx, "Selected cluster and node for run",
		slog.String("cluster_id", bestNode.ClusterID),
		slog.String("node_id", bestNode.ID),
		slog.Float64("score", bestScore(decision)),
	)

	return s.commitAssignment(ctx, &run, &agent, bestNode, decision)
}

func bestScore(d domain.SchedulingDecision) float64 {
	return 0.30*d.ResourceFit + 0.20*d.Latency + 0.15*d.Security + 0.15*d.Priority + 0.10*d.Locality + 0.10*d.Cost
}

func (s *Service) commitAssignment(ctx context.Context, run *domain.AgentRun, agent *domain.Agent, node domain.Node, decision domain.SchedulingDecision) error {
	traceID, _ := observability.GetTraceAndSpanID(ctx)
	corrID := observability.GetCorrelationID(ctx)
	fencingToken := uuid.NewString()
	gen := 0
	if run.RecoveryGeneration > 0 {
		gen = run.RecoveryGeneration
	}

	return s.txManager.WithinTx(ctx, func(txCtx context.Context) error {
		assignment := &domain.SchedulingAssignment{
			RunID:               run.ID,
			ClusterID:           node.ClusterID,
			NodeID:              node.ID,
			AlgorithmVersion:    "deterministic-v1",
			ExecutionGeneration: gen,
			FencingToken:        fencingToken,
			Score:               bestScore(decision),
			Decision:            decision,
			CreatedAt:           time.Now(),
			UpdatedAt:           time.Now(),
			Version:             1,
		}

		created, err := s.assignmentRepo.Assign(txCtx, assignment)
		if err != nil {
			return fmt.Errorf("assignment conflict: %w", err)
		}
		if !created {
			slog.InfoContext(txCtx, "Run already scheduled, skipping duplicate", slog.String("run_id", run.ID))
			return nil
		}

		// Update run state with cluster, node, generation, and fencing token
		run.State = types.StateScheduled
		run.Node = node.ID
		run.Cluster = node.ClusterID
		run.RecoveryGeneration = gen
		run.FencingToken = fencingToken
		if err := s.runRepo.Update(txCtx, *run); err != nil {
			return fmt.Errorf("failed to update run state: %w", err)
		}

		// Emit Outbox event with execution generation and fencing token
		payload := events.RunScheduledPayload{
			RunID:               run.ID,
			AgentID:             agent.ID,
			ClusterID:           node.ClusterID,
			NodeID:              node.ID,
			AlgorithmVersion:    assignment.AlgorithmVersion,
			ExecutionGeneration: gen,
			FencingToken:        fencingToken,
			FinalScore:          assignment.Score,
			Scores: map[string]float64{
				"resource_fit": assignment.Decision.ResourceFit,
				"latency":      assignment.Decision.Latency,
				"security":     assignment.Decision.Security,
				"priority":     assignment.Decision.Priority,
				"locality":     assignment.Decision.Locality,
				"cost":         assignment.Decision.Cost,
			},
			AgentImage:  agent.Image,
			AgentCPU:    agent.Resources.CPU,
			AgentMemory: agent.Resources.Memory,
		}

		payloadBytes, _ := json.Marshal(payload)

		// 1. Generic Subject
		event := events.Event{
			EventID:       uuid.New().String(),
			EventType:     events.SubjectRunScheduled,
			SchemaVersion: 1,
			AggregateType: "Run",
			AggregateID:   run.ID,
			TenantID:      agent.TenantID,
			CorrelationID: corrID,
			TraceParent:   traceID,
			OccurredAt:    time.Now(),
			Payload:       json.RawMessage(payloadBytes),
		}
		if err := s.outboxRepo.Insert(txCtx, event); err != nil {
			return fmt.Errorf("failed to insert scheduled event into outbox: %w", err)
		}

		// 2. Cluster-Targeted Subject: sentinel.run.v1.scheduled.<cluster_id>
		if node.ClusterID != "" {
			clusterTargetedEvent := events.Event{
				EventID:       uuid.New().String(),
				EventType:     events.SubjectRunScheduledForCluster(node.ClusterID),
				SchemaVersion: 1,
				AggregateType: "Run",
				AggregateID:   run.ID,
				TenantID:      agent.TenantID,
				CorrelationID: corrID,
				TraceParent:   traceID,
				OccurredAt:    time.Now(),
				Payload:       json.RawMessage(payloadBytes),
			}
			if err := s.outboxRepo.Insert(txCtx, clusterTargetedEvent); err != nil {
				return fmt.Errorf("failed to insert cluster-targeted scheduled event: %w", err)
			}
		}

		return nil
	})
}

func (s *Service) failScheduling(ctx context.Context, run *domain.AgentRun, agent *domain.Agent, reason string, isTransient bool) error {
	return s.txManager.WithinTx(ctx, func(txCtx context.Context) error {
		// Even if failed, we transition state to FAILED for structural errors
		if !isTransient {
			run.State = types.StateFailed
			run.FailureReason = fmt.Sprintf("Scheduling Failed: %s", reason)
			if err := s.runRepo.Update(txCtx, *run); err != nil {
				return err
			}
		}

		payload := events.RunSchedulingFailedPayload{
			RunID:       run.ID,
			AgentID:     agent.ID,
			Reason:      reason,
			IsTransient: isTransient,
		}

		payloadBytes, _ := json.Marshal(payload)

		event := events.Event{
			EventID:       uuid.New().String(),
			EventType:     events.SubjectRunSchedulingFailed,
			SchemaVersion: 1,
			AggregateType: "Run",
			AggregateID:   run.ID,
			TenantID:      agent.TenantID,
			OccurredAt:    time.Now(),
			Payload:       json.RawMessage(payloadBytes),
		}

		if err := s.outboxRepo.Insert(txCtx, event); err != nil {
			return fmt.Errorf("failed to insert scheduling failed event: %w", err)
		}

		return nil
	})
}

// RescheduleRequest encapsulates parameters needed to failover an interrupted run to a healthy cluster/node.
type RescheduleRequest struct {
	RunID              string                            `json:"run_id"`
	ExcludeClusterIDs  []string                          `json:"exclude_cluster_ids,omitempty"`
	ExcludeNodeIDs     []string                          `json:"exclude_node_ids,omitempty"`
	RecoveryGeneration int                               `json:"recovery_generation"`
	Checkpoint         *events.CheckpointMetadataPayload `json:"checkpoint,omitempty"`
}

// RescheduleRun executes deterministic placement for an interrupted run, excluding dead clusters/nodes and generating a new fencing token.
func (s *Service) RescheduleRun(ctx context.Context, req RescheduleRequest) error {
	ctx, span := observability.StartSpan(ctx, "scheduler.reschedule_decision")
	defer span.End()

	m := observability.GetMetrics()
	startTime := time.Now()

	logger := slog.With("run_id", req.RunID, "recovery_generation", req.RecoveryGeneration)

	run, err := s.runRepo.Get(ctx, req.RunID)
	if err != nil {
		observability.RecordSpanError(span, err)
		return fmt.Errorf("failed to get run: %w", err)
	}

	observability.InjectSpanAttributes(span, run.ID, run.AgentID, run.TenantID, observability.GetCorrelationID(ctx), req.RecoveryGeneration)

	// Validate recovery generation
	if req.RecoveryGeneration > 0 && req.RecoveryGeneration < run.RecoveryGeneration {
		logger.WarnContext(ctx, "Stale recovery request ignored", slog.Int("req_gen", req.RecoveryGeneration), slog.Int("current_gen", run.RecoveryGeneration))
		return nil
	}

	agent, err := s.agentRepo.Get(ctx, run.AgentID)
	if err != nil {
		observability.RecordSpanError(span, err)
		return fmt.Errorf("failed to get agent: %w", err)
	}

	var bestNode domain.Node
	var decision domain.SchedulingDecision
	var scoreErr error

	if s.clusterResourceProv != nil {
		clusters, err := s.clusterResourceProv.ListClusters(ctx)
		if err != nil {
			observability.RecordSpanError(span, err)
			return fmt.Errorf("failed to list clusters: %w", err)
		}

		bestCluster, clusterDec, clusterErr := SelectBestCluster(&agent, clusters, req.ExcludeClusterIDs, s.scoringPolicy)
		if clusterErr != nil {
			scoreErr = clusterErr
		} else {
			nodes, nodeErr := s.clusterResourceProv.ListNodes(ctx, bestCluster.ID)
			if nodeErr != nil {
				scoreErr = nodeErr
			} else {
				validNodes := filterNodesExcluding(&agent, nodes, req.ExcludeNodeIDs)
				bestNode, decision, scoreErr = scoreNodes(&agent, validNodes)
				decision.CandidatesConsidered = len(nodes)
				decision.CandidatesRejected = len(nodes) - len(validNodes)
				if bestNode.ClusterID == "" {
					bestNode.ClusterID = bestCluster.ID
				}
				span.SetAttributes(
					attribute.String("scheduler.tier1_failover_cluster", bestCluster.ID),
					attribute.Float64("scheduler.tier1_cluster_score", clusterDec.FinalScore),
				)
			}
		}
	} else {
		nodes, err := s.resourceProv.ListNodes(ctx)
		if err != nil {
			observability.RecordSpanError(span, err)
			return fmt.Errorf("failed to list nodes: %w", err)
		}

		validNodes := filterNodesExcluding(&agent, nodes, req.ExcludeNodeIDs)
		bestNode, decision, scoreErr = scoreNodes(&agent, validNodes)
		decision.CandidatesConsidered = len(nodes)
		decision.CandidatesRejected = len(nodes) - len(validNodes)
	}

	m.SchedulerCandidatesTotal.WithLabelValues("fit").Add(float64(decision.CandidatesConsidered - decision.CandidatesRejected))
	if decision.CandidatesRejected > 0 {
		m.SchedulerRejectionsTotal.WithLabelValues("filter_mismatch").Add(float64(decision.CandidatesRejected))
	}

	duration := time.Since(startTime).Seconds()
	m.SchedulerDecisionDurationSec.WithLabelValues("deterministic-v1").Observe(duration)

	if scoreErr != nil {
		logger.WarnContext(ctx, "Rescheduling failed: no healthy candidate clusters/nodes fit", slog.String("reason", scoreErr.Error()))
		m.SchedulerDecisionsTotal.WithLabelValues("FAILURE", "deterministic-v1").Inc()
		observability.RecordSpanError(span, scoreErr)
		return s.failScheduling(ctx, &run, &agent, scoreErr.Error(), false)
	}

	m.SchedulerDecisionsTotal.WithLabelValues("SUCCESS", "deterministic-v1").Inc()
	span.SetAttributes(
		attribute.String("scheduler.replacement_cluster_id", bestNode.ClusterID),
		attribute.String("scheduler.replacement_node_id", bestNode.ID),
		attribute.Float64("scheduler.final_score", bestScore(decision)),
		attribute.Int("scheduler.recovery_generation", req.RecoveryGeneration),
	)

	logger.InfoContext(ctx, "Selected replacement cluster and node for recovered run",
		slog.String("cluster_id", bestNode.ClusterID),
		slog.String("node_id", bestNode.ID),
		slog.Float64("score", bestScore(decision)),
	)

	return s.commitReassignment(ctx, &run, &agent, bestNode, decision, req)
}

func (s *Service) commitReassignment(
	ctx context.Context,
	run *domain.AgentRun,
	agent *domain.Agent,
	node domain.Node,
	decision domain.SchedulingDecision,
	req RescheduleRequest,
) error {
	traceID, _ := observability.GetTraceAndSpanID(ctx)
	corrID := observability.GetCorrelationID(ctx)
	newFencingToken := uuid.NewString()

	return s.txManager.WithinTx(ctx, func(txCtx context.Context) error {
		gen := req.RecoveryGeneration
		if gen == 0 {
			gen = run.RecoveryGeneration + 1
		}

		assignment := &domain.SchedulingAssignment{
			RunID:               run.ID,
			ClusterID:           node.ClusterID,
			NodeID:              node.ID,
			AlgorithmVersion:    "deterministic-v1",
			ExecutionGeneration: gen,
			FencingToken:        newFencingToken,
			Score:               bestScore(decision),
			Decision:            decision,
			CreatedAt:           time.Now(),
			UpdatedAt:           time.Now(),
			Version:             1,
		}

		if err := s.assignmentRepo.Reassign(txCtx, assignment); err != nil {
			return fmt.Errorf("failed to reassign: %w", err)
		}

		// Update run state for recovery with new generation and fencing token
		run.State = types.StateScheduled
		run.Node = node.ID
		run.Cluster = node.ClusterID
		run.RetryCount++
		run.RecoveryGeneration = gen
		run.FencingToken = newFencingToken
		if req.Checkpoint != nil {
			run.RecoveredFromCheckpointID = req.Checkpoint.ID
			run.LastCheckpointID = req.Checkpoint.ID
		}

		if err := s.runRepo.Update(txCtx, *run); err != nil {
			return fmt.Errorf("failed to update recovered run state: %w", err)
		}

		// Emit Outbox event with Checkpoint restore metadata and new fencing token
		payload := events.RunScheduledPayload{
			RunID:               run.ID,
			AgentID:             agent.ID,
			ClusterID:           node.ClusterID,
			NodeID:              node.ID,
			AlgorithmVersion:    assignment.AlgorithmVersion,
			ExecutionGeneration: gen,
			FencingToken:        newFencingToken,
			FinalScore:          assignment.Score,
			Scores: map[string]float64{
				"resource_fit": assignment.Decision.ResourceFit,
				"latency":      assignment.Decision.Latency,
				"security":     assignment.Decision.Security,
				"priority":     assignment.Decision.Priority,
				"locality":     assignment.Decision.Locality,
				"cost":         assignment.Decision.Cost,
			},
			AgentImage:  agent.Image,
			AgentCPU:    agent.Resources.CPU,
			AgentMemory: agent.Resources.Memory,
			Checkpoint:  req.Checkpoint,
		}

		payloadBytes, _ := json.Marshal(payload)

		// 1. Generic Subject
		event := events.Event{
			EventID:       uuid.New().String(),
			EventType:     events.SubjectRunScheduled,
			SchemaVersion: 1,
			AggregateType: "Run",
			AggregateID:   run.ID,
			TenantID:      agent.TenantID,
			CorrelationID: corrID,
			TraceParent:   traceID,
			OccurredAt:    time.Now(),
			Payload:       json.RawMessage(payloadBytes),
		}
		if err := s.outboxRepo.Insert(txCtx, event); err != nil {
			return fmt.Errorf("failed to insert recovered scheduled event: %w", err)
		}

		// 2. Cluster-Targeted Subject: sentinel.run.v1.scheduled.<cluster_id>
		if node.ClusterID != "" {
			clusterTargetedEvent := events.Event{
				EventID:       uuid.New().String(),
				EventType:     events.SubjectRunScheduledForCluster(node.ClusterID),
				SchemaVersion: 1,
				AggregateType: "Run",
				AggregateID:   run.ID,
				TenantID:      agent.TenantID,
				CorrelationID: corrID,
				TraceParent:   traceID,
				OccurredAt:    time.Now(),
				Payload:       json.RawMessage(payloadBytes),
			}
			if err := s.outboxRepo.Insert(txCtx, clusterTargetedEvent); err != nil {
				return fmt.Errorf("failed to insert cluster-targeted recovered event: %w", err)
			}
		}

		return nil
	})
}
