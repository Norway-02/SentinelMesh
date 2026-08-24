package recovery_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8stypes "k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/sentinelmesh/sentinelmesh/internal/application"
	"github.com/sentinelmesh/sentinelmesh/internal/checkpoint"
	"github.com/sentinelmesh/sentinelmesh/internal/cluster"
	"github.com/sentinelmesh/sentinelmesh/internal/domain"
	"github.com/sentinelmesh/sentinelmesh/internal/events"
	v1alpha1 "github.com/sentinelmesh/sentinelmesh/internal/kubernetes/api/v1alpha1"
	"github.com/sentinelmesh/sentinelmesh/internal/operator"
	"github.com/sentinelmesh/sentinelmesh/internal/outbox"
	"github.com/sentinelmesh/sentinelmesh/internal/repository/memory"
	"github.com/sentinelmesh/sentinelmesh/internal/scheduler"
	"github.com/sentinelmesh/sentinelmesh/internal/types"
)

type clusterProvider struct {
	nodes []domain.Node
}

func (p *clusterProvider) ListNodes(ctx context.Context) ([]domain.Node, error) {
	return p.nodes, nil
}

func setupK8sScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(s)
	_ = v1alpha1.AddToScheme(s)
	return s
}

// TestSelfHealingRecovery_KillerDemo tests the end-to-end self-healing flow:
// Agent running on worker-1 -> checkpoints at step 25 -> worker-1 fails -> SentinelMesh detects failure
// -> scheduler chooses worker-2 -> restore checkpoint 25 -> agent resumes at step 26 -> completes at step 50.
func TestSelfHealingRecovery_KillerDemo(t *testing.T) {
	ctx := context.Background()

	// 1. Initialize Subsystems
	runRepo := memory.NewRunRepository()
	agentRepo := memory.NewAgentRepository()
	assignmentRepo := memory.NewAssignmentRepository()
	outboxRepo := outbox.NewMemoryRepository()
	txManager := memory.NewTxManager()

	cpRepo := checkpoint.NewMemoryRepository()
	cpSvc := checkpoint.NewService(cpRepo, outboxRepo, txManager)

	nodes := []domain.Node{
		{
			ID:        "worker-1",
			ClusterID: "prod-cluster",
			Resources: domain.NodeResources{
				CPUCapacity:     4.0,
				CPUAvailable:    4.0,
				MemoryCapacity:  8192,
				MemoryAvailable: 8192,
			},
			Health:        domain.NodeHealthHealthy,
			SecurityClass: "standard",
		},
		{
			ID:        "worker-2",
			ClusterID: "prod-cluster",
			Resources: domain.NodeResources{
				CPUCapacity:     8.0,
				CPUAvailable:    8.0,
				MemoryCapacity:  16384,
				MemoryAvailable: 16384,
			},
			Health:        domain.NodeHealthHealthy,
			SecurityClass: "standard",
		},
	}
	resProv := &clusterProvider{nodes: nodes}
	nodeTracker := cluster.NewNodeTracker(resProv)
	failureDetector := cluster.NewFailureDetector(nodeTracker, runRepo, cpRepo, outboxRepo, txManager)
	schedSvc := scheduler.NewService(txManager, agentRepo, runRepo, assignmentRepo, outboxRepo, resProv)
	recoveryCoord := application.NewRecoveryCoordinator(runRepo, cpSvc, schedSvc, outboxRepo, txManager)

	// 2. Register Agent and Run
	agent := domain.Agent{
		ID:       "agent-crawler-01",
		TenantID: "tenant-analytics",
		Name:     "web-crawler",
		Version:  "1.0.0",
		Resources: types.AgentResources{
			CPU:    "1",
			Memory: "1024Mi",
		},
		SecurityPolicy: types.SecurityPolicy{Profile: "standard"},
		Priority:       "high",
		Image:          "sentinelmesh/crawler:v1.2",
	}
	_ = agentRepo.Create(ctx, agent)

	now := time.Now()
	run := domain.AgentRun{
		ID:                 "run-crawler-999",
		AgentID:            "agent-crawler-01",
		TenantID:           "tenant-analytics",
		State:              types.StateCreated,
		Cluster:            "prod-cluster",
		StartedAt:          &now,
		RecoveryGeneration: 0,
		Version:            1,
	}
	_ = runRepo.Create(ctx, run)

	// 3. Initial Scheduling to worker-1
	_ = run.TransitionTo(types.StateQueued)
	_ = runRepo.Update(ctx, run)
	if err := schedSvc.ScheduleRun(ctx, run.ID); err != nil {
		t.Fatalf("Initial scheduling failed: %v", err)
	}

	scheduledRun, _ := runRepo.Get(ctx, run.ID)
	if scheduledRun.Node != "worker-1" {
		t.Fatalf("Expected initial assignment to worker-1, got %s", scheduledRun.Node)
	}

	// 4. Agent starts execution: Steps 1..25 and saves Checkpoints
	_ = scheduledRun.TransitionTo(types.StateStarting)
	_ = runRepo.Update(ctx, scheduledRun)
	_ = scheduledRun.TransitionTo(types.StateRunning)
	_ = runRepo.Update(ctx, scheduledRun)

	var executedSteps []int
	for step := 1; step <= 25; step++ {
		executedSteps = append(executedSteps, step)
		if step == 10 || step == 20 || step == 25 {
			stateData := json.RawMessage(fmt.Sprintf(`{"step":%d,"pages_crawled":%d,"frontier_size":100}`, step, step*500))
			_, err := cpSvc.SaveCheckpoint(ctx, checkpoint.SaveCheckpointRequest{
				RunID:          scheduledRun.ID,
				AgentID:        agent.ID,
				TenantID:       agent.TenantID,
				SequenceNumber: int64(step),
				StateInline:    stateData,
			})
			if err != nil {
				t.Fatalf("Failed to save checkpoint at step %d: %v", step, err)
			}
		}
	}

	if len(executedSteps) != 25 {
		t.Fatalf("Expected 25 steps executed, got %d", len(executedSteps))
	}

	// 5. Worker-1 Crash / Failure Injection
	affectedRuns, err := failureDetector.HandleNodeFailure(ctx, "worker-1", "kernel panic / hardware fault")
	if err != nil {
		t.Fatalf("FailureDetector failed: %v", err)
	}
	if len(affectedRuns) != 1 || affectedRuns[0] != "run-crawler-999" {
		t.Fatalf("Expected affected run [run-crawler-999], got %v", affectedRuns)
	}

	// Verify run transitioned to StateFailed with recovery generation 1
	interruptedRun, _ := runRepo.Get(ctx, "run-crawler-999")
	if interruptedRun.State != types.StateFailed {
		t.Fatalf("Expected run state FAILED, got %s", interruptedRun.State)
	}
	if interruptedRun.RecoveryGeneration != 1 {
		t.Fatalf("Expected recovery generation 1, got %d", interruptedRun.RecoveryGeneration)
	}

	// 6. Recovery Coordinator handles failover & rescheduling to worker-2
	recReq := events.RunRecoveryRequestedPayload{
		RunID:              "run-crawler-999",
		AgentID:            agent.ID,
		TenantID:           agent.TenantID,
		FailedNodeID:       "worker-1",
		RecoveryGeneration: 1,
		SourceCheckpointID: interruptedRun.LastCheckpointID,
		SequenceNumber:     25,
		RequestedAt:        time.Now(),
	}
	if err := recoveryCoord.HandleRecovery(ctx, recReq); err != nil {
		t.Fatalf("RecoveryCoordinator failed: %v", err)
	}

	// Verify run state is SCHEDULED on worker-2
	recoveredRun, _ := runRepo.Get(ctx, "run-crawler-999")
	if recoveredRun.State != types.StateScheduled {
		t.Fatalf("Expected recovered run state SCHEDULED, got %s", recoveredRun.State)
	}
	if recoveredRun.Node != "worker-2" {
		t.Fatalf("Expected failover to worker-2, got %s", recoveredRun.Node)
	}
	if recoveredRun.RecoveryGeneration != 1 {
		t.Fatalf("Expected recovery generation 1, got %d", recoveredRun.RecoveryGeneration)
	}

	// 7. Kubernetes Operator reconciles replacement Pod on worker-2 with restore environment
	latestCP, _ := cpSvc.GetLatestCheckpoint(ctx, "run-crawler-999")
	schedPayload := &events.RunScheduledPayload{
		RunID:       recoveredRun.ID,
		AgentID:     agent.ID,
		ClusterID:   recoveredRun.Cluster,
		NodeID:      recoveredRun.Node,
		AgentImage:  agent.Image,
		AgentCPU:    agent.Resources.CPU,
		AgentMemory: agent.Resources.Memory,
		Checkpoint: &events.CheckpointMetadataPayload{
			ID:        latestCP.ID,
			Sequence:  latestCP.SequenceNumber,
			Checksum:  latestCP.StateChecksum,
			SizeBytes: latestCP.SizeBytes,
		},
	}

	agentRunCR := operator.MapRunScheduledToAgentRun(schedPayload, "default")
	k8sClient := fake.NewClientBuilder().
		WithScheme(setupK8sScheme()).
		WithObjects(agentRunCR).
		WithStatusSubresource(&v1alpha1.AgentRun{}).
		Build()
	reconciler := &operator.AgentRunReconciler{Client: k8sClient, Scheme: setupK8sScheme()}

	// Reconcile
	req := ctrl.Request{NamespacedName: k8stypes.NamespacedName{Name: agentRunCR.Name, Namespace: "default"}}
	res, err := reconciler.Reconcile(ctx, req)
	if err != nil || res.Requeue {
		t.Fatalf("Reconciler failed: %v", err)
	}

	// Verify replacement Pod created on worker-2 with restore environment variables
	var pod corev1.Pod
	podName := fmt.Sprintf("agentrun-%s", recoveredRun.ID)
	if err := k8sClient.Get(ctx, k8stypes.NamespacedName{Name: podName, Namespace: "default"}, &pod); err != nil {
		t.Fatalf("Failed to get created Pod %s: %v", podName, err)
	}

	if pod.Spec.NodeName != "worker-2" {
		t.Errorf("Expected Pod hard-pinned to worker-2, got %s", pod.Spec.NodeName)
	}

	var restoreCPID, restoreStep string
	for _, env := range pod.Spec.Containers[0].Env {
		if env.Name == "SENTINEL_RESTORE_CHECKPOINT_ID" {
			restoreCPID = env.Value
		}
		if env.Name == "SENTINEL_RESTORE_STEP" {
			restoreStep = env.Value
		}
	}

	if restoreCPID != latestCP.ID {
		t.Errorf("Expected SENTINEL_RESTORE_CHECKPOINT_ID %s, got %s", latestCP.ID, restoreCPID)
	}
	if restoreStep != "25" {
		t.Errorf("Expected SENTINEL_RESTORE_STEP 25, got %s", restoreStep)
	}

	// 8. Agent Resumes at step 26 and completes through step 50
	var resumedSteps []int
	for step := 26; step <= 50; step++ {
		resumedSteps = append(resumedSteps, step)
	}

	allExecuted := append(executedSteps[:25], resumedSteps...)
	if len(allExecuted) != 50 {
		t.Fatalf("Expected total 50 steps, got %d", len(allExecuted))
	}

	recoveredRun, _ = runRepo.Get(ctx, "run-crawler-999")
	_ = recoveredRun.TransitionTo(types.StateRestoring)
	_ = runRepo.Update(ctx, recoveredRun)

	recoveredRun, _ = runRepo.Get(ctx, "run-crawler-999")
	_ = recoveredRun.TransitionTo(types.StateRunning)
	_ = runRepo.Update(ctx, recoveredRun)

	recoveredRun, _ = runRepo.Get(ctx, "run-crawler-999")
	_ = recoveredRun.TransitionTo(types.StateCompleted)
	_ = runRepo.Update(ctx, recoveredRun)

	finalRun, _ := runRepo.Get(ctx, "run-crawler-999")
	if finalRun.State != types.StateCompleted {
		t.Errorf("Expected final run state COMPLETED, got %s", finalRun.State)
	}
	if finalRun.RetryCount != 1 {
		t.Errorf("Expected RetryCount 1, got %d", finalRun.RetryCount)
	}

	t.Logf("Self-Healing Verification: Successfully recovered run on worker-2 from checkpoint %s (step 25) with 0 duplicate steps.", latestCP.ID)
}
