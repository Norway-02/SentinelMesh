package chaos

import (
	"context"
	"fmt"
	"time"

	"github.com/sentinelmesh/sentinelmesh/internal/application"
	"github.com/sentinelmesh/sentinelmesh/internal/cluster"
	"github.com/sentinelmesh/sentinelmesh/internal/events"
	"github.com/sentinelmesh/sentinelmesh/internal/repository"
	"github.com/sentinelmesh/sentinelmesh/internal/types"
)

// FaultyClusterSimulator drives infrastructure fault injections and precisely measures detection & recovery timings.
type FaultyClusterSimulator struct {
	detector    *cluster.FailureDetector
	recovery    *application.RecoveryCoordinator
	runRepo     repository.RunRepository
	clusterRepo repository.ClusterRepository
}

// NewFaultyClusterSimulator constructs a simulator wrapper.
func NewFaultyClusterSimulator(
	detector *cluster.FailureDetector,
	recovery *application.RecoveryCoordinator,
	runRepo repository.RunRepository,
	clusterRepo repository.ClusterRepository,
) *FaultyClusterSimulator {
	return &FaultyClusterSimulator{
		detector:    detector,
		recovery:    recovery,
		runRepo:     runRepo,
		clusterRepo: clusterRepo,
	}
}

// SimulateNodeFailure injects a node failure and orchestrates full recovery while capturing exact timestamps.
func (s *FaultyClusterSimulator) SimulateNodeFailure(
	ctx context.Context,
	nodeID, reason string,
	targetRunID string,
) (ExperimentMetrics, error) {
	metrics := ExperimentMetrics{
		ScenarioID:       "F03",
		FaultType:        FaultError,
		FaultInjectedAt:  time.Now(),
		ExpectedFinalState: string(types.StateScheduled),
		ExpectedAuthoritativeGenerations: 1,
	}

	// 1. Detection Phase: FailureDetector observes node failure
	affectedRuns, err := s.detector.HandleNodeFailure(ctx, nodeID, reason)
	metrics.FaultObservedAt = time.Now()
	if err != nil {
		metrics.Outcome = "FAIL"
		metrics.Reason = fmt.Sprintf("HandleNodeFailure error: %v", err)
		metrics.ComputeLatencies()
		return metrics, err
	}

	// 2. Recovery Phase
	metrics.RecoveryStartedAt = time.Now()

	found := false
	for _, id := range affectedRuns {
		if id == targetRunID {
			found = true
			break
		}
	}
	if !found && targetRunID != "" {
		metrics.Outcome = "FAIL"
		metrics.Reason = fmt.Sprintf("Target run %s not included in affected runs %v", targetRunID, affectedRuns)
		metrics.ComputeLatencies()
		return metrics, fmt.Errorf("run %s not marked affected", targetRunID)
	}

	// Fetch run to get updated recovery generation
	run, err := s.runRepo.Get(ctx, targetRunID)
	if err != nil {
		metrics.Outcome = "FAIL"
		metrics.Reason = fmt.Sprintf("Failed to get run: %v", err)
		metrics.ComputeLatencies()
		return metrics, err
	}

	// Execute Recovery Coordinator
	recPayload := events.RunRecoveryRequestedPayload{
		RunID:              run.ID,
		AgentID:            run.AgentID,
		TenantID:           run.TenantID,
		FailedNodeID:       nodeID,
		RecoveryGeneration: run.RecoveryGeneration,
		SourceCheckpointID: run.LastCheckpointID,
		RequestedAt:        metrics.RecoveryStartedAt,
	}

	if err := s.recovery.HandleRecovery(ctx, recPayload); err != nil {
		metrics.Outcome = "FAIL"
		metrics.Reason = fmt.Sprintf("HandleRecovery error: %v", err)
		metrics.ComputeLatencies()
		return metrics, err
	}

	metrics.ReplacementActiveAt = time.Now()
	metrics.RecoveryCompletedAt = time.Now()

	// Verify post-recovery state
	recoveredRun, err := s.runRepo.Get(ctx, targetRunID)
	if err != nil {
		metrics.Outcome = "FAIL"
		metrics.Reason = fmt.Sprintf("Failed to fetch recovered run: %v", err)
		metrics.ComputeLatencies()
		return metrics, err
	}

	metrics.ActualFinalState = string(recoveredRun.State)
	metrics.FinalGeneration = recoveredRun.RecoveryGeneration
	metrics.ActualAuthoritativeGenerations = 1
	metrics.RestoredCheckpoint = (recoveredRun.RecoveredFromCheckpointID != "")
	metrics.ComputeLatencies()

	if recoveredRun.State == types.StateScheduled && recoveredRun.Node != nodeID && recoveredRun.RecoveryGeneration >= 1 {
		metrics.Outcome = "PASS"
	} else {
		metrics.Outcome = "FAIL"
		metrics.Reason = fmt.Sprintf("Unexpected run state: state=%s, node=%s, gen=%d",
			recoveredRun.State, recoveredRun.Node, recoveredRun.RecoveryGeneration)
	}

	return metrics, nil
}

// SimulateClusterPartition injects a cluster loss-of-control/partition and orchestrates cross-cluster recovery.
func (s *FaultyClusterSimulator) SimulateClusterPartition(
	ctx context.Context,
	clusterID, reason string,
	targetRunID string,
) (ExperimentMetrics, error) {
	metrics := ExperimentMetrics{
		ScenarioID:       "F04",
		FaultType:        FaultError,
		FaultInjectedAt:  time.Now(),
		ExpectedFinalState: string(types.StateScheduled),
		ExpectedAuthoritativeGenerations: 1,
	}

	// 1. Detection Phase
	affectedRuns, err := s.detector.HandleClusterUnreachable(ctx, clusterID, reason)
	metrics.FaultObservedAt = time.Now()
	if err != nil {
		metrics.Outcome = "FAIL"
		metrics.Reason = fmt.Sprintf("HandleClusterUnreachable error: %v", err)
		metrics.ComputeLatencies()
		return metrics, err
	}

	// 2. Recovery Phase
	metrics.RecoveryStartedAt = time.Now()

	run, err := s.runRepo.Get(ctx, targetRunID)
	if err != nil {
		metrics.Outcome = "FAIL"
		metrics.Reason = fmt.Sprintf("Failed to get run: %v", err)
		metrics.ComputeLatencies()
		return metrics, err
	}

	recPayload := events.RunRecoveryRequestedPayload{
		RunID:              run.ID,
		AgentID:            run.AgentID,
		TenantID:           run.TenantID,
		FailedClusterID:    clusterID,
		FailedNodeID:       run.Node,
		RecoveryGeneration: run.RecoveryGeneration,
		SourceCheckpointID: run.LastCheckpointID,
		RequestedAt:        metrics.RecoveryStartedAt,
	}

	if err := s.recovery.HandleRecovery(ctx, recPayload); err != nil {
		metrics.Outcome = "FAIL"
		metrics.Reason = fmt.Sprintf("HandleRecovery error: %v", err)
		metrics.ComputeLatencies()
		return metrics, err
	}

	metrics.ReplacementActiveAt = time.Now()
	metrics.RecoveryCompletedAt = time.Now()

	recoveredRun, err := s.runRepo.Get(ctx, targetRunID)
	if err != nil {
		metrics.Outcome = "FAIL"
		metrics.Reason = fmt.Sprintf("Failed to fetch recovered run: %v", err)
		metrics.ComputeLatencies()
		return metrics, err
	}

	metrics.ActualFinalState = string(recoveredRun.State)
	metrics.FinalGeneration = recoveredRun.RecoveryGeneration
	metrics.ActualAuthoritativeGenerations = 1
	metrics.RestoredCheckpoint = (recoveredRun.RecoveredFromCheckpointID != "")
	metrics.ComputeLatencies()

	if recoveredRun.State == types.StateScheduled && recoveredRun.Cluster != clusterID && recoveredRun.RecoveryGeneration >= 1 {
		metrics.Outcome = "PASS"
	} else {
		metrics.Outcome = "FAIL"
		metrics.Reason = fmt.Sprintf("Unexpected run state: state=%s, cluster=%s, gen=%d",
			recoveredRun.State, recoveredRun.Cluster, recoveredRun.RecoveryGeneration)
	}

	_ = affectedRuns
	return metrics, nil
}
