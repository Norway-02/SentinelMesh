package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/sentinelmesh/sentinelmesh/internal/checkpoint"
	"github.com/sentinelmesh/sentinelmesh/internal/events"
	"github.com/sentinelmesh/sentinelmesh/internal/outbox"
	"github.com/sentinelmesh/sentinelmesh/internal/repository"
	"github.com/sentinelmesh/sentinelmesh/internal/types"
)

// FailureDetector detects infrastructure failures and initiates recovery for affected agent workloads.
type FailureDetector struct {
	mu                  sync.Mutex
	tracker             *NodeTracker
	runRepo             repository.RunRepository
	checkpointRepo      checkpoint.Repository
	outboxRepo          outbox.Repository
	txManager           repository.TxManager
	processedRecoveries map[string]int // runID -> last recovery generation
}

// NewFailureDetector constructs a FailureDetector.
func NewFailureDetector(
	tracker *NodeTracker,
	runRepo repository.RunRepository,
	checkpointRepo checkpoint.Repository,
	outboxRepo outbox.Repository,
	txManager repository.TxManager,
) *FailureDetector {
	return &FailureDetector{
		tracker:             tracker,
		runRepo:             runRepo,
		checkpointRepo:      checkpointRepo,
		outboxRepo:          outboxRepo,
		txManager:           txManager,
		processedRecoveries: make(map[string]int),
	}
}

// DetectAndHandleFailures synchronizes node health and triggers recovery for any interrupted runs.
func (d *FailureDetector) DetectAndHandleFailures(ctx context.Context) ([]string, error) {
	newlyFailed, err := d.tracker.Sync(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to sync node health: %w", err)
	}

	var recoveredRuns []string
	for _, nodeID := range newlyFailed {
		runs, err := d.HandleNodeFailure(ctx, nodeID, "node became NotReady / unreachable")
		if err != nil {
			return nil, err
		}
		recoveredRuns = append(recoveredRuns, runs...)
	}

	return recoveredRuns, nil
}

// HandleNodeFailure explicitly processes failure on a specific node.
func (d *FailureDetector) HandleNodeFailure(ctx context.Context, nodeID, reason string) ([]string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.tracker.MarkNodeFailed(nodeID, reason)

	runs, err := d.runRepo.ListByNode(ctx, nodeID)
	if err != nil {
		return nil, fmt.Errorf("failed to list runs for node %s: %w", nodeID, err)
	}

	var affectedRunIDs []string
	for _, r := range runs {
		if r.State == types.StateRunning || r.State == types.StateStarting ||
			r.State == types.StateScheduled || r.State == types.StateQueued ||
			r.State == types.StateCheckpointing {
			affectedRunIDs = append(affectedRunIDs, r.ID)
		}
	}

	now := time.Now()

	// 1. Emit ClusterNodeFailed event
	if d.outboxRepo != nil {
		nodeFailedPayload, _ := json.Marshal(events.ClusterNodeFailedPayload{
			ClusterID:      "local",
			NodeID:         nodeID,
			Reason:         reason,
			FailedAt:       now,
			AffectedRunIDs: affectedRunIDs,
		})

		_ = d.outboxRepo.Insert(ctx, events.Event{
			EventID:       uuid.NewString(),
			EventType:     events.SubjectClusterNodeFailed,
			SchemaVersion: 1,
			AggregateType: "Cluster",
			AggregateID:   nodeID,
			TenantID:      "system",
			OccurredAt:    now,
			Payload:       nodeFailedPayload,
		})
	}

	// 2. Process each affected run
	for _, r := range runs {
		if r.State != types.StateRunning && r.State != types.StateStarting &&
			r.State != types.StateScheduled && r.State != types.StateQueued &&
			r.State != types.StateCheckpointing {
			continue
		}

		gen := r.RecoveryGeneration + 1
		if lastGen, exists := d.processedRecoveries[r.ID]; exists && lastGen >= gen {
			continue
		}
		d.processedRecoveries[r.ID] = gen

		var sourceCPID string
		var seqNum int64
		if d.checkpointRepo != nil {
			cp, err := d.checkpointRepo.GetLatest(ctx, r.ID)
			if err == nil && cp != nil && cp.VerifyIntegrity() {
				sourceCPID = cp.ID
				seqNum = cp.SequenceNumber
			}
		}

		r.FailureReason = fmt.Sprintf("node failure: %s (%s)", nodeID, reason)
		r.RecoveryGeneration = gen
		if sourceCPID != "" {
			r.LastCheckpointID = sourceCPID
		}
		_ = r.TransitionTo(types.StateFailed)
		_ = d.runRepo.Update(ctx, r)

		if d.outboxRepo != nil {
			recPayload, _ := json.Marshal(events.RunRecoveryRequestedPayload{
				RunID:              r.ID,
				AgentID:            r.AgentID,
				TenantID:           r.TenantID,
				FailedNodeID:       nodeID,
				RecoveryGeneration: gen,
				SourceCheckpointID: sourceCPID,
				SequenceNumber:     seqNum,
				RequestedAt:        now,
			})

			_ = d.outboxRepo.Insert(ctx, events.Event{
				EventID:       uuid.NewString(),
				EventType:     events.SubjectRunRecoveryRequested,
				SchemaVersion: 1,
				AggregateType: "Run",
				AggregateID:   r.ID,
				TenantID:      r.TenantID,
				OccurredAt:    now,
				Payload:       recPayload,
			})
		}
	}

	return affectedRunIDs, nil
}

// HandleClusterUnreachable processes total cluster partition or loss-of-contact, initiating cross-cluster recovery.
func (d *FailureDetector) HandleClusterUnreachable(ctx context.Context, clusterID, reason string) ([]string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	runs, err := d.runRepo.ListByCluster(ctx, clusterID)
	if err != nil {
		return nil, fmt.Errorf("failed to list runs for cluster %s: %w", clusterID, err)
	}

	var affectedRunIDs []string
	for _, r := range runs {
		if r.State == types.StateRunning || r.State == types.StateStarting ||
			r.State == types.StateScheduled || r.State == types.StateQueued ||
			r.State == types.StateCheckpointing {
			affectedRunIDs = append(affectedRunIDs, r.ID)
		}
	}

	now := time.Now()

	// 1. Emit ClusterUnreachable event
	if d.outboxRepo != nil {
		unreachablePayload, _ := json.Marshal(events.ClusterUnreachablePayload{
			ClusterID:      clusterID,
			Reason:         reason,
			UnreachableAt:  now,
			AffectedRunIDs: affectedRunIDs,
		})

		_ = d.outboxRepo.Insert(ctx, events.Event{
			EventID:       uuid.NewString(),
			EventType:     events.SubjectClusterUnreachable,
			SchemaVersion: 1,
			AggregateType: "Cluster",
			AggregateID:   clusterID,
			TenantID:      "system",
			OccurredAt:    now,
			Payload:       unreachablePayload,
		})
	}

	// 2. Process each affected run on unreachable cluster
	for _, r := range runs {
		if r.State != types.StateRunning && r.State != types.StateStarting &&
			r.State != types.StateScheduled && r.State != types.StateQueued &&
			r.State != types.StateCheckpointing {
			continue
		}

		gen := r.RecoveryGeneration + 1
		if lastGen, exists := d.processedRecoveries[r.ID]; exists && lastGen >= gen {
			continue
		}
		d.processedRecoveries[r.ID] = gen

		var sourceCPID string
		var seqNum int64
		if d.checkpointRepo != nil {
			cp, err := d.checkpointRepo.GetLatest(ctx, r.ID)
			if err == nil && cp != nil && cp.VerifyIntegrity() {
				sourceCPID = cp.ID
				seqNum = cp.SequenceNumber
			}
		}

		r.FailureReason = fmt.Sprintf("cluster unreachable: %s (%s)", clusterID, reason)
		r.RecoveryGeneration = gen
		if sourceCPID != "" {
			r.LastCheckpointID = sourceCPID
		}
		_ = r.TransitionTo(types.StateFailed)
		_ = d.runRepo.Update(ctx, r)

		if d.outboxRepo != nil {
			recPayload, _ := json.Marshal(events.RunRecoveryRequestedPayload{
				RunID:              r.ID,
				AgentID:            r.AgentID,
				TenantID:           r.TenantID,
				FailedClusterID:    clusterID,
				FailedNodeID:       r.Node,
				RecoveryGeneration: gen,
				SourceCheckpointID: sourceCPID,
				SequenceNumber:     seqNum,
				RequestedAt:        now,
			})

			_ = d.outboxRepo.Insert(ctx, events.Event{
				EventID:       uuid.NewString(),
				EventType:     events.SubjectRunRecoveryRequested,
				SchemaVersion: 1,
				AggregateType: "Run",
				AggregateID:   r.ID,
				TenantID:      r.TenantID,
				OccurredAt:    now,
				Payload:       recPayload,
			})
		}
	}

	return affectedRunIDs, nil
}
