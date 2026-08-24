package chaos_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/sentinelmesh/sentinelmesh/internal/application"
	"github.com/sentinelmesh/sentinelmesh/internal/chaos"
	"github.com/sentinelmesh/sentinelmesh/internal/checkpoint"
	"github.com/sentinelmesh/sentinelmesh/internal/cluster"
	"github.com/sentinelmesh/sentinelmesh/internal/domain"
	v1alpha1 "github.com/sentinelmesh/sentinelmesh/internal/kubernetes/api/v1alpha1"
	"github.com/sentinelmesh/sentinelmesh/internal/operator"
	"github.com/sentinelmesh/sentinelmesh/internal/outbox"
	"github.com/sentinelmesh/sentinelmesh/internal/repository"
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
		return nil, fmt.Errorf("cluster %s not found", clusterID)
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

// ChaosHarness holds the complete SentinelMesh domain and service graph wrapped with chaos injection.
type ChaosHarness struct {
	RawRunRepo        *memory.RunRepository
	RawOutboxRepo     *outbox.MemoryRepository
	RawCheckpointRepo *checkpoint.MemoryRepository
	RawClusterRepo    *memory.ClusterRepository
	AgentRepo         *memory.AgentRepository
	AssignmentRepo    repository.AssignmentRepository
	TxManager         *memory.TxManager

	FaultController   *chaos.FaultController
	FaultyRunRepo     *chaos.FaultyRunRepository
	FaultyOutboxRepo  *chaos.FaultyOutboxRepository
	FaultyCPRepo      *chaos.FaultyCheckpointRepository
	ClusterSimulator  *chaos.FaultyClusterSimulator

	SchedulerSvc      *scheduler.Service
	CheckpointSvc     *checkpoint.Service
	RecoveryCoord     *application.RecoveryCoordinator
	FailureDetector   *cluster.FailureDetector
	ClusterRegistry   *cluster.ClusterRegistry
	NodeTracker       *cluster.NodeTracker

	K8sClient         client.Client
	Reconciler        *operator.AgentRunReconciler
}

type staticNodeProvider struct {
	nodes []domain.Node
}

func (p *staticNodeProvider) ListNodes(ctx context.Context) ([]domain.Node, error) {
	return p.nodes, nil
}

func buildChaosHarness(t *testing.T, seed int64, faults []chaos.FaultSpec) *ChaosHarness {
	ctx := context.Background()

	rawRunRepo := memory.NewRunRepository()
	rawOutboxRepo := outbox.NewMemoryRepository()
	rawCheckpointRepo := checkpoint.NewMemoryRepository()
	rawClusterRepo := memory.NewClusterRepository()
	agentRepo := memory.NewAgentRepository()
	assignmentRepo := memory.NewAssignmentRepository()
	txManager := memory.NewTxManager()

	controller := chaos.NewFaultController(seed, faults)
	faultyRunRepo := chaos.NewFaultyRunRepository(rawRunRepo, controller)
	faultyOutboxRepo := chaos.NewFaultyOutboxRepository(rawOutboxRepo, controller)
	faultyCPRepo := chaos.NewFaultyCheckpointRepository(rawCheckpointRepo, controller)

	clusters := []domain.Cluster{
		{
			ID:              "us-east-k8s",
			Name:            "US East Cluster",
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
			ID:              "eu-west-k8s",
			Name:            "EU West Cluster",
			Region:          "eu-west-1",
			ProviderType:    domain.ProviderKubernetes,
			SecurityClasses: []string{"standard"},
			NetworkCost:     1.2,
			BaseLatencyMs:   30.0,
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
		"us-east-k8s": {
			{ID: "us-worker-1", ClusterID: "us-east-k8s", Resources: domain.NodeResources{CPUCapacity: 500, CPUAvailable: 400, MemoryCapacity: 1000000, MemoryAvailable: 800000}, Health: domain.NodeHealthHealthy},
			{ID: "us-worker-2", ClusterID: "us-east-k8s", Resources: domain.NodeResources{CPUCapacity: 500, CPUAvailable: 400, MemoryCapacity: 1000000, MemoryAvailable: 800000}, Health: domain.NodeHealthHealthy},
		},
		"eu-west-k8s": {
			{ID: "eu-worker-1", ClusterID: "eu-west-k8s", Resources: domain.NodeResources{CPUCapacity: 500, CPUAvailable: 375, MemoryCapacity: 1000000, MemoryAvailable: 750000}, Health: domain.NodeHealthHealthy},
			{ID: "eu-worker-2", ClusterID: "eu-west-k8s", Resources: domain.NodeResources{CPUCapacity: 500, CPUAvailable: 375, MemoryCapacity: 1000000, MemoryAvailable: 750000}, Health: domain.NodeHealthHealthy},
		},
	}

	for _, c := range clusters {
		_ = rawClusterRepo.Register(ctx, c)
	}

	prov := newMemoryMultiClusterProvider(clusters, nodes)
	schedSvc := scheduler.NewService(txManager, agentRepo, faultyRunRepo, assignmentRepo, faultyOutboxRepo, nil).
		WithClusterResourceProvider(prov, scheduler.DefaultClusterScoringPolicy())

	cpSvc := checkpoint.NewService(faultyCPRepo, faultyOutboxRepo, txManager)
	recoveryCoord := application.NewRecoveryCoordinator(faultyRunRepo, cpSvc, schedSvc, faultyOutboxRepo, txManager)

	var allNodes []domain.Node
	for _, nl := range nodes {
		allNodes = append(allNodes, nl...)
	}
	nodeTracker := cluster.NewNodeTracker(&staticNodeProvider{nodes: allNodes})

	failureDetector := cluster.NewFailureDetector(nodeTracker, faultyRunRepo, faultyCPRepo, faultyOutboxRepo, txManager)
	clusterRegistry := cluster.NewClusterRegistry(rawClusterRepo, faultyOutboxRepo)
	clusterSim := chaos.NewFaultyClusterSimulator(failureDetector, recoveryCoord, faultyRunRepo, rawClusterRepo)

	k8sClient := fake.NewClientBuilder().
		WithScheme(setupScheme()).
		WithStatusSubresource(&v1alpha1.AgentRun{}).
		Build()
	reconciler := &operator.AgentRunReconciler{Client: k8sClient, Scheme: setupScheme()}

	return &ChaosHarness{
		RawRunRepo:        rawRunRepo,
		RawOutboxRepo:     rawOutboxRepo,
		RawCheckpointRepo: rawCheckpointRepo,
		RawClusterRepo:    rawClusterRepo,
		AgentRepo:         agentRepo,
		AssignmentRepo:    assignmentRepo,
		TxManager:         txManager,
		FaultController:   controller,
		FaultyRunRepo:     faultyRunRepo,
		FaultyOutboxRepo:  faultyOutboxRepo,
		FaultyCPRepo:      faultyCPRepo,
		ClusterSimulator:  clusterSim,
		SchedulerSvc:      schedSvc,
		CheckpointSvc:     cpSvc,
		RecoveryCoord:     recoveryCoord,
		FailureDetector:   failureDetector,
		ClusterRegistry:   clusterRegistry,
		NodeTracker:       nodeTracker,
		K8sClient:         k8sClient,
		Reconciler:        reconciler,
	}
}

func (h *ChaosHarness) CreateStandardAgent(ctx context.Context, agentID, tenantID string) error {
	return h.AgentRepo.Create(ctx, domain.Agent{
		ID:             agentID,
		TenantID:       tenantID,
		Name:           "chaos-agent-" + agentID,
		Version:        "1.0.0",
		Resources:      types.AgentResources{CPU: "1", Memory: "1024Mi"},
		SecurityPolicy: types.SecurityPolicy{Profile: "standard"},
		Image:          "sentinelmesh/agent:v1",
	})
}
