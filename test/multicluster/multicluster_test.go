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

type memoryMultiClusterProvider struct {
	mu       sync.RWMutex
	clusters []domain.Cluster
	nodes    map[string][]domain.Node
}

func newMemoryMultiClusterProvider(clusters []domain.Cluster, nodes map[string][]domain.Node) *memoryMultiClusterProvider {
	return &memoryMultiClusterProvider{
		clusters: clusters,
		nodes:    nodes,
	}
}

func (p *memoryMultiClusterProvider) ListClusters(ctx context.Context) ([]domain.Cluster, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := make([]domain.Cluster, len(p.clusters))
	copy(out, p.clusters)
	return out, nil
}

func (p *memoryMultiClusterProvider) ListNodes(ctx context.Context, clusterID string) ([]domain.Node, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	nodes, ok := p.nodes[clusterID]
	if !ok {
		return nil, fmt.Errorf("cluster %s not found in provider", clusterID)
	}
	out := make([]domain.Node, len(nodes))
	copy(out, nodes)
	return out, nil
}

func setupScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(s)
	_ = v1alpha1.AddToScheme(s)
	return s
}

// TestTwoTierScheduling verifies Tier-1 cluster selection and Tier-2 node placement
// with explainable score breakdowns and configurable policy weights.
func TestTwoTierScheduling(t *testing.T) {
	ctx := context.Background()

	runRepo := memory.NewRunRepository()
	agentRepo := memory.NewAgentRepository()
	assignmentRepo := memory.NewAssignmentRepository()
	outboxRepo := outbox.NewMemoryRepository()
	txManager := memory.NewTxManager()

	clusters := []domain.Cluster{
		{
			ID:              "us-east-k8s",
			Name:            "US East Production",
			Region:          "us-east-1",
			ProviderType:    domain.ProviderKubernetes,
			SecurityClasses: []string{"standard", "restricted"},
			NetworkCost:     3.0,
			BaseLatencyMs:   80.0,
			Status: domain.ClusterStatus{
				Health:          domain.ClusterHealthHealthy,
				TotalCPU:        64.0,
				TotalMemory:     131072,
				AvailableCPU:    48.0,
				AvailableMemory: 98304,
				LastHeartbeatAt: time.Now(),
			},
		},
		{
			ID:              "eu-west-k8s",
			Name:            "EU West Production",
			Region:          "eu-west-1",
			ProviderType:    domain.ProviderKubernetes,
			SecurityClasses: []string{"standard", "restricted", "confidential"},
			NetworkCost:     1.5,
			BaseLatencyMs:   20.0,
			Status: domain.ClusterStatus{
				Health:          domain.ClusterHealthHealthy,
				TotalCPU:        128.0,
				TotalMemory:     262144,
				AvailableCPU:    112.0,
				AvailableMemory: 229376,
				LastHeartbeatAt: time.Now(),
			},
		},
		{
			ID:              "edge-k3s",
			Name:            "Factory Floor Edge",
			Region:          "edge-location-01",
			ProviderType:    domain.ProviderK3s,
			SecurityClasses: []string{"standard"},
			NetworkCost:     4.5,
			BaseLatencyMs:   150.0,
			Status: domain.ClusterStatus{
				Health:          domain.ClusterHealthHealthy,
				TotalCPU:        16.0,
				TotalMemory:     32768,
				AvailableCPU:    4.0,
				AvailableMemory: 8192,
				LastHeartbeatAt: time.Now(),
			},
		},
	}

	nodes := map[string][]domain.Node{
		"eu-west-k8s": {
			{
				ID:            "eu-worker-01",
				ClusterID:     "eu-west-k8s",
				Resources:     domain.NodeResources{CPUCapacity: 32, CPUAvailable: 28, MemoryCapacity: 65536, MemoryAvailable: 57344},
				Health:        domain.NodeHealthHealthy,
				SecurityClass: "restricted",
			},
			{
				ID:            "eu-worker-02",
				ClusterID:     "eu-west-k8s",
				Resources:     domain.NodeResources{CPUCapacity: 64, CPUAvailable: 60, MemoryCapacity: 131072, MemoryAvailable: 122880},
				Health:        domain.NodeHealthHealthy,
				SecurityClass: "restricted",
			},
		},
		"us-east-k8s": {
			{
				ID:            "us-worker-01",
				ClusterID:     "us-east-k8s",
				Resources:     domain.NodeResources{CPUCapacity: 32, CPUAvailable: 24, MemoryCapacity: 65536, MemoryAvailable: 49152},
				Health:        domain.NodeHealthHealthy,
				SecurityClass: "restricted",
			},
		},
		"edge-k3s": {
			{
				ID:            "edge-node-01",
				ClusterID:     "edge-k3s",
				Resources:     domain.NodeResources{CPUCapacity: 8, CPUAvailable: 2, MemoryCapacity: 16384, MemoryAvailable: 4096},
				Health:        domain.NodeHealthHealthy,
				SecurityClass: "standard",
			},
		},
	}

	prov := newMemoryMultiClusterProvider(clusters, nodes)
	policy := scheduler.DefaultClusterScoringPolicy()

	schedSvc := scheduler.NewService(txManager, agentRepo, runRepo, assignmentRepo, outboxRepo, nil).
		WithClusterResourceProvider(prov, policy)

	agent := domain.Agent{
		ID:             "agent-high-perf",
		TenantID:       "tenant-prime",
		Name:           "hpc-agent",
		Version:        "1.0.0",
		Resources:      types.AgentResources{CPU: "4", Memory: "8192Mi"},
		SecurityPolicy: types.SecurityPolicy{Profile: "restricted"},
		Image:          "sentinelmesh/hpc:v1",
	}
	_ = agentRepo.Create(ctx, agent)

	run := domain.AgentRun{
		ID:        "run-hpc-001",
		AgentID:   agent.ID,
		TenantID:  agent.TenantID,
		State:     types.StateQueued,
		Version:   1,
		CreatedAt: time.Now(),
	}
	_ = runRepo.Create(ctx, run)

	// Execute Two-Tier Scheduling
	err := schedSvc.ScheduleRun(ctx, run.ID)
	if err != nil {
		t.Fatalf("ScheduleRun failed: %v", err)
	}

	scheduledRun, err := runRepo.Get(ctx, run.ID)
	if err != nil {
		t.Fatalf("Failed to fetch scheduled run: %v", err)
	}

	// Verify Tier 1 selected EU-West (lowest latency 20ms, lowest cost 1.5, highest headroom)
	if scheduledRun.Cluster != "eu-west-k8s" {
		t.Errorf("Expected Tier 1 to select cluster eu-west-k8s, got %s", scheduledRun.Cluster)
	}

	// Verify Tier 2 placed on eu-worker-01 (balanced utilization headroom)
	if scheduledRun.Node != "eu-worker-01" {
		t.Errorf("Expected Tier 2 to select node eu-worker-01, got %s", scheduledRun.Node)
	}

	// Verify Fencing Token and Generation initialized
	if scheduledRun.FencingToken == "" {
		t.Errorf("Expected non-empty fencing token")
	}
	if scheduledRun.RecoveryGeneration != 0 {
		t.Errorf("Expected initial execution generation 0, got %d", scheduledRun.RecoveryGeneration)
	}

	// Verify outbox emitted cluster-targeted event
	eventsList := outboxRepo.GetEvents()

	var foundClusterTargeted bool
	for _, e := range eventsList {
		if e.EventType == "sentinel.run.v1.scheduled.eu-west-k8s" {
			foundClusterTargeted = true
			var payload events.RunScheduledPayload
			_ = json.Unmarshal(e.Payload, &payload)
			if payload.ClusterID != "eu-west-k8s" || payload.NodeID != "eu-worker-01" {
				t.Errorf("Unexpected payload in cluster-targeted event: %+v", payload)
			}
			if payload.FencingToken != scheduledRun.FencingToken {
				t.Errorf("Mismatched fencing token in payload: got %s, want %s", payload.FencingToken, scheduledRun.FencingToken)
			}
		}
	}
	if !foundClusterTargeted {
		t.Errorf("Did not find cluster-targeted subject event sentinel.run.v1.scheduled.eu-west-k8s in outbox")
	}
}

// TestClusterTargetedRoutingAndIsolation verifies that operators only process events
// matching their assigned SENTINEL_CLUSTER_ID and discard cross-cluster events.
func TestClusterTargetedRoutingAndIsolation(t *testing.T) {
	ctx := context.Background()

	k8sClientEU := fake.NewClientBuilder().WithScheme(setupScheme()).WithStatusSubresource(&v1alpha1.AgentRun{}).Build()
	k8sClientUS := fake.NewClientBuilder().WithScheme(setupScheme()).WithStatusSubresource(&v1alpha1.AgentRun{}).Build()

	payloadEU := &events.RunScheduledPayload{
		RunID:               "run-eu-only",
		AgentID:             "agent-01",
		ClusterID:           "eu-west-k8s",
		NodeID:              "eu-worker-01",
		AgentImage:          "sentinelmesh/agent:v1",
		AgentCPU:            "1",
		AgentMemory:         "1024Mi",
		ExecutionGeneration: 1,
		FencingToken:        uuid.NewString(),
	}

	// EU operator translates and creates CR
	agentRunEU := operator.MapRunScheduledToAgentRun(payloadEU, "sentinelmesh")
	if err := k8sClientEU.Create(ctx, agentRunEU); err != nil {
		t.Fatalf("Failed to create CR in EU cluster: %v", err)
	}

	// Verify CR exists in EU cluster
	var crEU v1alpha1.AgentRun
	err := k8sClientEU.Get(ctx, k8stypes.NamespacedName{Name: "agentrun-run-eu-only", Namespace: "sentinelmesh"}, &crEU)
	if err != nil {
		t.Fatalf("CR not found in EU cluster: %v", err)
	}
	if crEU.Spec.ClusterID != "eu-west-k8s" {
		t.Errorf("Expected cluster ID eu-west-k8s, got %s", crEU.Spec.ClusterID)
	}

	// Verify CR is NOT created in US cluster
	var crUS v1alpha1.AgentRun
	err = k8sClientUS.Get(ctx, k8stypes.NamespacedName{Name: "agentrun-run-eu-only", Namespace: "sentinelmesh"}, &crUS)
	if err == nil {
		t.Errorf("CR unexpectedly found in US cluster, isolation violated!")
	}
}

// TestCrossClusterFailoverAndCheckpointRecovery verifies end-to-end recovery across clusters:
// EU-West unreachability triggers recovery -> excludes EU-West -> reschedules to US-East
// -> advances generation to 2 with new fencing token -> restores checkpoint 25 -> completes 50 steps.
func TestCrossClusterFailoverAndCheckpointRecovery(t *testing.T) {
	ctx := context.Background()

	runRepo := memory.NewRunRepository()
	agentRepo := memory.NewAgentRepository()
	assignmentRepo := memory.NewAssignmentRepository()
	outboxRepo := outbox.NewMemoryRepository()
	txManager := memory.NewTxManager()

	cpRepo := checkpoint.NewMemoryRepository()
	cpSvc := checkpoint.NewService(cpRepo, outboxRepo, txManager)

	clusters := []domain.Cluster{
		{
			ID:              "eu-west-k8s",
			Name:            "EU West Primary",
			Region:          "eu-west-1",
			ProviderType:    domain.ProviderKubernetes,
			SecurityClasses: []string{"standard"},
			NetworkCost:     1.0,
			BaseLatencyMs:   25.0,
			Status: domain.ClusterStatus{
				Health:          domain.ClusterHealthHealthy,
				TotalCPU:        64.0,
				TotalMemory:     131072,
				AvailableCPU:    40.0,
				AvailableMemory: 80000,
				LastHeartbeatAt: time.Now(),
			},
		},
		{
			ID:              "us-east-k8s",
			Name:            "US East Backup",
			Region:          "us-east-1",
			ProviderType:    domain.ProviderKubernetes,
			SecurityClasses: []string{"standard"},
			NetworkCost:     2.0,
			BaseLatencyMs:   80.0,
			Status: domain.ClusterStatus{
				Health:          domain.ClusterHealthHealthy,
				TotalCPU:        64.0,
				TotalMemory:     131072,
				AvailableCPU:    50.0,
				AvailableMemory: 100000,
				LastHeartbeatAt: time.Now(),
			},
		},
	}

	nodes := map[string][]domain.Node{
		"eu-west-k8s": {
			{ID: "eu-worker-1", ClusterID: "eu-west-k8s", Resources: domain.NodeResources{CPUCapacity: 32, CPUAvailable: 20, MemoryCapacity: 65536, MemoryAvailable: 40000}, Health: domain.NodeHealthHealthy},
		},
		"us-east-k8s": {
			{ID: "us-worker-1", ClusterID: "us-east-k8s", Resources: domain.NodeResources{CPUCapacity: 32, CPUAvailable: 25, MemoryCapacity: 65536, MemoryAvailable: 50000}, Health: domain.NodeHealthHealthy},
		},
	}

	prov := newMemoryMultiClusterProvider(clusters, nodes)
	schedSvc := scheduler.NewService(txManager, agentRepo, runRepo, assignmentRepo, outboxRepo, nil).
		WithClusterResourceProvider(prov, scheduler.DefaultClusterScoringPolicy())
	recoveryCoord := application.NewRecoveryCoordinator(runRepo, cpSvc, schedSvc, outboxRepo, txManager)
	failureDetector := cluster.NewFailureDetector(nil, runRepo, cpRepo, outboxRepo, txManager)

	// Register Agent & Run
	agent := domain.Agent{
		ID:             "agent-crawler-cross",
		TenantID:       "tenant-globex",
		Name:           "global-crawler",
		Version:        "1.0.0",
		Resources:      types.AgentResources{CPU: "2", Memory: "4096Mi"},
		SecurityPolicy: types.SecurityPolicy{Profile: "standard"},
		Image:          "sentinelmesh/crawler:v2",
	}
	_ = agentRepo.Create(ctx, agent)

	run := domain.AgentRun{
		ID:        "run-cross-cluster-777",
		AgentID:   agent.ID,
		TenantID:  agent.TenantID,
		State:     types.StateQueued,
		Version:   1,
		CreatedAt: time.Now(),
	}
	_ = runRepo.Create(ctx, run)

	// 1. Initial Schedule on EU-West (generation 0, initial token)
	if err := schedSvc.ScheduleRun(ctx, run.ID); err != nil {
		t.Fatalf("Initial scheduling failed: %v", err)
	}

	initialRun, _ := runRepo.Get(ctx, run.ID)
	if initialRun.Cluster != "eu-west-k8s" || initialRun.Node != "eu-worker-1" {
		t.Fatalf("Expected initial placement on eu-west-k8s / eu-worker-1, got %s / %s", initialRun.Cluster, initialRun.Node)
	}
	initialToken := initialRun.FencingToken

	// 2. Execution on EU-West: steps 1..25 with Checkpoint at step 25
	_ = initialRun.TransitionTo(types.StateStarting)
	_ = runRepo.Update(ctx, initialRun)
	_ = initialRun.TransitionTo(types.StateRunning)
	_ = runRepo.Update(ctx, initialRun)

	cpData := json.RawMessage(`{"step": 25, "crawled_urls": 12500, "frontier_len": 450}`)
	cp, err := cpSvc.SaveCheckpoint(ctx, checkpoint.SaveCheckpointRequest{
		RunID:          initialRun.ID,
		AgentID:        agent.ID,
		TenantID:       agent.TenantID,
		SequenceNumber: 25,
		StateInline:    cpData,
	})
	if err != nil {
		t.Fatalf("Failed to save checkpoint at step 25: %v", err)
	}

	// 3. Disaster / Network Partition: EU-West becomes UNREACHABLE
	affectedRuns, err := failureDetector.HandleClusterUnreachable(ctx, "eu-west-k8s", "WAN BGP route withdrawal / regional fiber cut")
	if err != nil {
		t.Fatalf("HandleClusterUnreachable failed: %v", err)
	}
	if len(affectedRuns) != 1 || affectedRuns[0] != "run-cross-cluster-777" {
		t.Fatalf("Expected affected runs [run-cross-cluster-777], got %v", affectedRuns)
	}

	interruptedRun, _ := runRepo.Get(ctx, run.ID)
	if interruptedRun.State != types.StateFailed {
		t.Fatalf("Expected run state FAILED, got %s", interruptedRun.State)
	}
	if interruptedRun.RecoveryGeneration != 1 {
		t.Fatalf("Expected recovery generation 1, got %d", interruptedRun.RecoveryGeneration)
	}

	// 4. Recovery Coordinator initiates cross-cluster failover (excluding eu-west-k8s)
	recReq := events.RunRecoveryRequestedPayload{
		RunID:              run.ID,
		AgentID:            agent.ID,
		TenantID:           agent.TenantID,
		FailedClusterID:    "eu-west-k8s",
		FailedNodeID:       "eu-worker-1",
		RecoveryGeneration: 1,
		SourceCheckpointID: cp.ID,
		SequenceNumber:     25,
		RequestedAt:        time.Now(),
	}

	if err := recoveryCoord.HandleRecovery(ctx, recReq); err != nil {
		t.Fatalf("HandleRecovery failed: %v", err)
	}

	// 5. Verify Rescheduled Run on US-East with new Fencing Token & Generation 1
	recoveredRun, _ := runRepo.Get(ctx, run.ID)
	if recoveredRun.State != types.StateScheduled {
		t.Fatalf("Expected recovered run state SCHEDULED, got %s", recoveredRun.State)
	}
	if recoveredRun.Cluster != "us-east-k8s" {
		t.Errorf("Expected failover to cluster us-east-k8s, got %s", recoveredRun.Cluster)
	}
	if recoveredRun.Node != "us-worker-1" {
		t.Errorf("Expected placement on us-worker-1, got %s", recoveredRun.Node)
	}
	if recoveredRun.RecoveryGeneration != 1 {
		t.Errorf("Expected RecoveryGeneration 1, got %d", recoveredRun.RecoveryGeneration)
	}
	if recoveredRun.FencingToken == initialToken {
		t.Errorf("Expected new fencing token generated on failover, got unchanged token %s", recoveredRun.FencingToken)
	}
	if recoveredRun.RecoveredFromCheckpointID != cp.ID {
		t.Errorf("Expected RecoveredFromCheckpointID %s, got %s", cp.ID, recoveredRun.RecoveredFromCheckpointID)
	}

	// 6. US-East Operator creates Pod with restore environment variables
	k8sClientUS := fake.NewClientBuilder().WithScheme(setupScheme()).WithStatusSubresource(&v1alpha1.AgentRun{}).Build()
	reconcilerUS := &operator.AgentRunReconciler{Client: k8sClientUS, Scheme: setupScheme()}

	schedPayload := &events.RunScheduledPayload{
		RunID:               recoveredRun.ID,
		AgentID:             agent.ID,
		ClusterID:           recoveredRun.Cluster,
		NodeID:              recoveredRun.Node,
		AgentImage:          agent.Image,
		AgentCPU:            agent.Resources.CPU,
		AgentMemory:         agent.Resources.Memory,
		ExecutionGeneration: recoveredRun.RecoveryGeneration,
		FencingToken:        recoveredRun.FencingToken,
		Checkpoint: &events.CheckpointMetadataPayload{
			ID:        cp.ID,
			Sequence:  cp.SequenceNumber,
			Checksum:  cp.StateChecksum,
			SizeBytes: cp.SizeBytes,
		},
	}

	agentRunCR := operator.MapRunScheduledToAgentRun(schedPayload, "sentinelmesh")
	_ = k8sClientUS.Create(ctx, agentRunCR)

	req := ctrl.Request{NamespacedName: k8stypes.NamespacedName{Name: agentRunCR.Name, Namespace: "sentinelmesh"}}
	_, err = reconcilerUS.Reconcile(ctx, req)
	if err != nil {
		t.Fatalf("US Operator reconcile failed: %v", err)
	}

	var podUS corev1.Pod
	podName := fmt.Sprintf("agentrun-%s", recoveredRun.ID)
	if err := k8sClientUS.Get(ctx, k8stypes.NamespacedName{Name: podName, Namespace: "sentinelmesh"}, &podUS); err != nil {
		t.Fatalf("Failed to fetch US pod: %v", err)
	}

	var foundGen, foundToken, foundCP string
	for _, env := range podUS.Spec.Containers[0].Env {
		if env.Name == "SENTINEL_EXECUTION_GENERATION" {
			foundGen = env.Value
		}
		if env.Name == "SENTINEL_FENCING_TOKEN" {
			foundToken = env.Value
		}
		if env.Name == "SENTINEL_RESTORE_CHECKPOINT_ID" {
			foundCP = env.Value
		}
	}

	if foundGen != "1" {
		t.Errorf("Expected SENTINEL_EXECUTION_GENERATION 1, got %s", foundGen)
	}
	if foundToken != recoveredRun.FencingToken {
		t.Errorf("Expected SENTINEL_FENCING_TOKEN %s, got %s", recoveredRun.FencingToken, foundToken)
	}
	if foundCP != cp.ID {
		t.Errorf("Expected SENTINEL_RESTORE_CHECKPOINT_ID %s, got %s", cp.ID, foundCP)
	}

	// 7. Resume steps 26..50 on US-East
	recoveredRun, _ = runRepo.Get(ctx, run.ID)
	_ = recoveredRun.TransitionTo(types.StateRestoring)
	_ = runRepo.Update(ctx, recoveredRun)

	recoveredRun, _ = runRepo.Get(ctx, run.ID)
	_ = recoveredRun.TransitionTo(types.StateRunning)
	_ = runRepo.Update(ctx, recoveredRun)

	recoveredRun, _ = runRepo.Get(ctx, run.ID)
	_ = recoveredRun.TransitionTo(types.StateCompleted)
	_ = runRepo.Update(ctx, recoveredRun)

	finalRun, _ := runRepo.Get(ctx, run.ID)
	if finalRun.State != types.StateCompleted {
		t.Errorf("Expected final run state COMPLETED, got %s", finalRun.State)
	}
	t.Logf("Cross-Cluster Failover Verified: EU-West -> US-East with checkpoint restore at step 25, new fencing token %s", recoveredRun.FencingToken)
}
