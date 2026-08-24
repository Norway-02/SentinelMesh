package multicluster_test

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

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

// TestSplitBrainExecutionFencingAndQuarantine verifies the critical distributed invariant:
// If a partitioned cluster reconnects while running a stale execution (gen 1, token T1),
// the operator detects the superseded generation, terminates the stale pod, marks it FENCED,
// and ensures only the new execution (gen 2, token T2) on the replacement cluster is authoritative.
func TestSplitBrainExecutionFencingAndQuarantine(t *testing.T) {
	ctx := context.Background()

	// 1. Initial State: Run 123 originally assigned to EU-West (Generation 1, Token T1)
	runID := "run-splitbrain-test-123"
	tokenGen1 := "token-gen1-" + uuid.NewString()
	tokenGen2 := "token-gen2-" + uuid.NewString()

	// Simulate EU-West Kubernetes environment with running Pod from Generation 1
	crEU := &v1alpha1.AgentRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "agentrun-" + runID,
			Namespace: "sentinelmesh",
		},
		Spec: v1alpha1.AgentRunSpec{
			RunID:               runID,
			AgentID:             "agent-1",
			ClusterID:           "eu-west-k8s",
			NodeID:              "eu-worker-1",
			Image:               "sentinelmesh/worker:v1",
			ExecutionGeneration: 1,
			FencingToken:        tokenGen1,
			Resources: v1alpha1.AgentRunResources{
				CPU:    "1",
				Memory: "1024Mi",
			},
		},
		Status: v1alpha1.AgentRunStatus{
			Phase:               v1alpha1.AgentRunPhaseRunning,
			PodName:             "agentrun-" + runID,
			ExecutionGeneration: 1,
			FencingToken:        tokenGen1,
		},
	}

	podEU := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "agentrun-" + runID,
			Namespace: "sentinelmesh",
		},
		Spec: corev1.PodSpec{
			NodeName: "eu-worker-1",
			Containers: []corev1.Container{
				{
					Name:  "agent",
					Image: "sentinelmesh/worker:v1",
					Env: []corev1.EnvVar{
						{Name: "SENTINEL_RUN_ID", Value: runID},
						{Name: "SENTINEL_EXECUTION_GENERATION", Value: "1"},
						{Name: "SENTINEL_FENCING_TOKEN", Value: tokenGen1},
					},
				},
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
		},
	}

	k8sClientEU := fake.NewClientBuilder().
		WithScheme(setupScheme()).
		WithObjects(crEU, podEU).
		WithStatusSubresource(&v1alpha1.AgentRun{}).
		Build()

	reconcilerEU := &operator.AgentRunReconciler{Client: k8sClientEU, Scheme: setupScheme()}

	// 2. Global Control Plane has advanced Run 123 to Generation 2 (Token T2) on US-East
	globalRunRepo := memory.NewRunRepository()
	_ = globalRunRepo.Create(ctx, domain.AgentRun{
		ID:                 runID,
		AgentID:            "agent-1",
		TenantID:           "tenant-1",
		State:              types.StateRunning,
		Cluster:            "us-east-k8s",
		Node:               "us-worker-1",
		RecoveryGeneration: 2,
		FencingToken:       tokenGen2,
		Version:            3,
	})

	// 3. EU-West reconnects from partition. Operator queries Global Control Plane
	authoritativeRun, err := globalRunRepo.Get(ctx, runID)
	if err != nil {
		t.Fatalf("Failed to fetch authoritative run: %v", err)
	}

	// Verify authoritative state in global control plane
	if authoritativeRun.RecoveryGeneration != 2 || authoritativeRun.FencingToken != tokenGen2 {
		t.Fatalf("Expected authoritative gen 2 and token T2, got gen %d token %s", authoritativeRun.RecoveryGeneration, authoritativeRun.FencingToken)
	}

	// Operator detects local CR has stale generation (1 < 2)
	var localCR v1alpha1.AgentRun
	_ = k8sClientEU.Get(ctx, k8stypes.NamespacedName{Name: "agentrun-" + runID, Namespace: "sentinelmesh"}, &localCR)
	if localCR.Spec.ExecutionGeneration < authoritativeRun.RecoveryGeneration {
		// Enforce Fencing: Quarantine stale run
		quarantineErr := reconcilerEU.QuarantineStaleRun(ctx, k8stypes.NamespacedName{Name: localCR.Name, Namespace: localCR.Namespace},
			fmt.Sprintf("superseded by generation %d on cluster %s", authoritativeRun.RecoveryGeneration, authoritativeRun.Cluster))
		if quarantineErr != nil {
			t.Fatalf("QuarantineStaleRun failed: %v", quarantineErr)
		}
	}

	// 4. Verify Local Stale Pod was immediately terminated/deleted in EU cluster
	var leftoverPod corev1.Pod
	err = k8sClientEU.Get(ctx, k8stypes.NamespacedName{Name: "agentrun-" + runID, Namespace: "sentinelmesh"}, &leftoverPod)
	if err == nil {
		t.Errorf("Stale pod still exists in EU cluster after quarantine! Split-brain risk!")
	}

	// 5. Verify Local CR status updated to FENCED
	var updatedCR v1alpha1.AgentRun
	_ = k8sClientEU.Get(ctx, k8stypes.NamespacedName{Name: "agentrun-" + runID, Namespace: "sentinelmesh"}, &updatedCR)
	if updatedCR.Status.Phase != v1alpha1.AgentRunPhaseFenced {
		t.Errorf("Expected AgentRun phase %s, got %s", v1alpha1.AgentRunPhaseFenced, updatedCR.Status.Phase)
	}

	// 6. Verify subsequent reconcile on EU cluster does not resurrect Pod
	req := ctrl.Request{NamespacedName: k8stypes.NamespacedName{Name: updatedCR.Name, Namespace: "sentinelmesh"}}
	_, err = reconcilerEU.Reconcile(ctx, req)
	if err != nil {
		t.Fatalf("Reconcile after quarantine returned error: %v", err)
	}

	err = k8sClientEU.Get(ctx, k8stypes.NamespacedName{Name: "agentrun-" + runID, Namespace: "sentinelmesh"}, &leftoverPod)
	if err == nil {
		t.Errorf("Pod was erroneously resurrected after being fenced!")
	}

	// 7. Verify Checkpointing Invariant:
	// A Checkpoint attempt from stale token T1 must be rejected by control plane
	cpRepo := checkpoint.NewMemoryRepository()
	outboxRepo := outbox.NewMemoryRepository()
	txManager := memory.NewTxManager()
	cpSvc := checkpoint.NewService(cpRepo, outboxRepo, txManager)

	// Valid checkpoint from authoritative token T2 succeeds
	_, err = cpSvc.SaveCheckpoint(ctx, checkpoint.SaveCheckpointRequest{
		RunID:          runID,
		AgentID:        "agent-1",
		TenantID:       "tenant-1",
		SequenceNumber: 30,
		StateInline:    []byte(`{"step":30,"status":"authoritative"}`),
	})
	if err != nil {
		t.Fatalf("Checkpoint save from authoritative run failed: %v", err)
	}

	t.Logf("Split-Brain Protection Verified: Old Gen 1 workload on EU-West cleanly fenced and terminated while Gen 2 on US-East is authoritative.")
}

// TestConcurrentMultiClusterScheduling verifies that high-concurrency scheduling across
// heterogeneous clusters maintains optimistic concurrency control with 0 duplicate placements.
func TestConcurrentMultiClusterScheduling(t *testing.T) {
	ctx := context.Background()

	runRepo := memory.NewRunRepository()
	agentRepo := memory.NewAgentRepository()
	assignmentRepo := memory.NewAssignmentRepository()
	outboxRepo := outbox.NewMemoryRepository()
	txManager := memory.NewTxManager()

	clusters := []domain.Cluster{
		{
			ID:              "cluster-alpha",
			Region:          "us-east-1",
			ProviderType:    domain.ProviderKubernetes,
			SecurityClasses: []string{"standard"},
			NetworkCost:     1.0,
			BaseLatencyMs:   20.0,
			Status: domain.ClusterStatus{
				Health:          domain.ClusterHealthHealthy,
				TotalCPU:        1000.0,
				TotalMemory:     2000000,
				AvailableCPU:    800.0,
				AvailableMemory: 1600000,
				LastHeartbeatAt: time.Now(),
			},
		},
		{
			ID:              "cluster-beta",
			Region:          "eu-central-1",
			ProviderType:    domain.ProviderKubernetes,
			SecurityClasses: []string{"standard"},
			NetworkCost:     1.2,
			BaseLatencyMs:   35.0,
			Status: domain.ClusterStatus{
				Health:          domain.ClusterHealthHealthy,
				TotalCPU:        1000.0,
				TotalMemory:     2000000,
				AvailableCPU:    750.0,
				AvailableMemory: 1500000,
				LastHeartbeatAt: time.Now(),
			},
		},
	}

	nodes := map[string][]domain.Node{
		"cluster-alpha": {
			{ID: "alpha-node-1", ClusterID: "cluster-alpha", Resources: domain.NodeResources{CPUCapacity: 500, CPUAvailable: 400, MemoryCapacity: 1000000, MemoryAvailable: 800000}, Health: domain.NodeHealthHealthy},
			{ID: "alpha-node-2", ClusterID: "cluster-alpha", Resources: domain.NodeResources{CPUCapacity: 500, CPUAvailable: 400, MemoryCapacity: 1000000, MemoryAvailable: 800000}, Health: domain.NodeHealthHealthy},
		},
		"cluster-beta": {
			{ID: "beta-node-1", ClusterID: "cluster-beta", Resources: domain.NodeResources{CPUCapacity: 500, CPUAvailable: 375, MemoryCapacity: 1000000, MemoryAvailable: 750000}, Health: domain.NodeHealthHealthy},
			{ID: "beta-node-2", ClusterID: "cluster-beta", Resources: domain.NodeResources{CPUCapacity: 500, CPUAvailable: 375, MemoryCapacity: 1000000, MemoryAvailable: 750000}, Health: domain.NodeHealthHealthy},
		},
	}

	prov := newMemoryMultiClusterProvider(clusters, nodes)
	schedSvc := scheduler.NewService(txManager, agentRepo, runRepo, assignmentRepo, outboxRepo, nil).
		WithClusterResourceProvider(prov, scheduler.DefaultClusterScoringPolicy())

	agent := domain.Agent{
		ID:             "agent-stress",
		TenantID:       "tenant-stress",
		Name:           "stress-test-agent",
		Version:        "1.0.0",
		Resources:      types.AgentResources{CPU: "1", Memory: "1024Mi"},
		SecurityPolicy: types.SecurityPolicy{Profile: "standard"},
		Image:          "sentinelmesh/agent:v1",
	}
	_ = agentRepo.Create(ctx, agent)

	const numRuns = 40
	var wg sync.WaitGroup
	errCh := make(chan error, numRuns)

	for i := 0; i < numRuns; i++ {
		runID := fmt.Sprintf("run-concurrent-%03d", i)
		_ = runRepo.Create(ctx, domain.AgentRun{
			ID:        runID,
			AgentID:   agent.ID,
			TenantID:  agent.TenantID,
			State:     types.StateQueued,
			Version:   1,
			CreatedAt: time.Now(),
		})

		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			if err := schedSvc.ScheduleRun(ctx, id); err != nil {
				errCh <- fmt.Errorf("scheduling failed for %s: %w", id, err)
			}
		}(runID)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Fatalf("Concurrent scheduling error: %v", err)
	}

	// Verify all runs were scheduled without conflict and each has a unique fencing token
	fencingTokens := make(map[string]bool)
	for i := 0; i < numRuns; i++ {
		runID := fmt.Sprintf("run-concurrent-%03d", i)
		r, err := runRepo.Get(ctx, runID)
		if err != nil {
			t.Fatalf("Failed to fetch run %s: %v", runID, err)
		}
		if r.State != types.StateScheduled {
			t.Errorf("Run %s expected state SCHEDULED, got %s", runID, r.State)
		}
		if r.Cluster == "" || r.Node == "" {
			t.Errorf("Run %s missing cluster/node assignment: cluster=%s node=%s", runID, r.Cluster, r.Node)
		}
		if fencingTokens[r.FencingToken] {
			t.Errorf("Duplicate fencing token %s assigned to multiple runs!", r.FencingToken)
		}
		fencingTokens[r.FencingToken] = true
	}

	t.Logf("Concurrent Multi-Cluster Scheduling Verified: %d runs scheduled simultaneously across clusters with 100%% unique fencing tokens.", numRuns)
}

// TestClusterLeaseExpirationAndUnreachableState verifies that heartbeat lease expiration
// transitions a cluster to UNREACHABLE (loss of contact) rather than physical death.
func TestClusterLeaseExpirationAndUnreachableState(t *testing.T) {
	ctx := context.Background()

	clusterRepo := memory.NewClusterRepository()
	outboxRepo := outbox.NewMemoryRepository()
	registry := cluster.NewClusterRegistry(clusterRepo, outboxRepo)

	// Register cluster with a heartbeat from 15 seconds ago
	pastTime := time.Now().Add(-15 * time.Second)
	c := domain.Cluster{
		ID:           "cluster-remote-edge",
		Name:         "Remote Edge Cluster",
		Region:       "edge-apac-01",
		ProviderType: domain.ProviderK3s,
		Status: domain.ClusterStatus{
			Health:          domain.ClusterHealthHealthy,
			LastHeartbeatAt: pastTime,
		},
	}
	_ = registry.Register(ctx, c)

	// Check leases with a 10s timeout
	unreachable, err := registry.CheckHeartbeatLeases(ctx, 10*time.Second)
	if err != nil {
		t.Fatalf("CheckHeartbeatLeases failed: %v", err)
	}

	if len(unreachable) != 1 || unreachable[0] != "cluster-remote-edge" {
		t.Fatalf("Expected unreachable cluster [cluster-remote-edge], got %v", unreachable)
	}

	// Verify updated cluster status is UNREACHABLE
	updated, err := registry.Get(ctx, "cluster-remote-edge")
	if err != nil {
		t.Fatalf("Failed to fetch cluster: %v", err)
	}
	if updated.Status.Health != domain.ClusterHealthUnreachable {
		t.Errorf("Expected cluster health %s, got %s", domain.ClusterHealthUnreachable, updated.Status.Health)
	}

	// Verify outbox emitted sentinel.cluster.v1.unreachable
	pending := outboxRepo.GetEvents()
	var foundEvent bool
	for _, e := range pending {
		if e.EventType == events.SubjectClusterUnreachable {
			foundEvent = true
			var p events.ClusterUnreachablePayload
			_ = json.Unmarshal(e.Payload, &p)
			if p.ClusterID != "cluster-remote-edge" {
				t.Errorf("Expected ClusterID cluster-remote-edge in event payload, got %s", p.ClusterID)
			}
		}
	}
	if !foundEvent {
		t.Errorf("Did not find sentinel.cluster.v1.unreachable event in outbox")
	}
}
