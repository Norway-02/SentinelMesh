package scheduler

import (
	"context"
	"fmt"
	"sync"

	"github.com/sentinelmesh/sentinelmesh/internal/domain"
	"github.com/sentinelmesh/sentinelmesh/internal/repository"
)

// ClusterResourceProvider abstracts multi-cluster topology and per-cluster compute node state.
type ClusterResourceProvider interface {
	ListClusters(ctx context.Context) ([]domain.Cluster, error)
	ListNodes(ctx context.Context, clusterID string) ([]domain.Node, error)
}

// MultiClusterResourceProvider aggregates multiple cluster node providers backed by ClusterRepository.
type MultiClusterResourceProvider struct {
	mu          sync.RWMutex
	clusterRepo repository.ClusterRepository
	providers   map[string]ResourceProvider // clusterID -> ResourceProvider for that cluster
}

// NewMultiClusterResourceProvider constructs a MultiClusterResourceProvider.
func NewMultiClusterResourceProvider(clusterRepo repository.ClusterRepository) *MultiClusterResourceProvider {
	return &MultiClusterResourceProvider{
		clusterRepo: clusterRepo,
		providers:   make(map[string]ResourceProvider),
	}
}

// RegisterClusterProvider attaches a specific node provider to a cluster ID.
func (p *MultiClusterResourceProvider) RegisterClusterProvider(clusterID string, provider ResourceProvider) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.providers[clusterID] = provider
}

// ListClusters returns all registered clusters from the cluster repository.
func (p *MultiClusterResourceProvider) ListClusters(ctx context.Context) ([]domain.Cluster, error) {
	if p.clusterRepo == nil {
		return nil, fmt.Errorf("cluster repository is not configured")
	}
	return p.clusterRepo.List(ctx)
}

// ListNodes returns nodes for a specific cluster ID.
func (p *MultiClusterResourceProvider) ListNodes(ctx context.Context, clusterID string) ([]domain.Node, error) {
	p.mu.RLock()
	provider, exists := p.providers[clusterID]
	p.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("no resource provider registered for cluster %s", clusterID)
	}

	nodes, err := provider.ListNodes(ctx)
	if err != nil {
		return nil, err
	}

	// Ensure all returned nodes carry the correct clusterID
	for i := range nodes {
		if nodes[i].ClusterID == "" {
			nodes[i].ClusterID = clusterID
		}
	}
	return nodes, nil
}

// ListAllNodes implements standard ResourceProvider by aggregating nodes from all clusters.
func (p *MultiClusterResourceProvider) ListAllNodes(ctx context.Context) ([]domain.Node, error) {
	clusters, err := p.ListClusters(ctx)
	if err != nil {
		return nil, err
	}

	var allNodes []domain.Node
	for _, c := range clusters {
		if !c.Status.Health.IsAvailable() {
			continue
		}
		nodes, err := p.ListNodes(ctx, c.ID)
		if err != nil {
			// Skip or log unreachable cluster node listing
			continue
		}
		allNodes = append(allNodes, nodes...)
	}
	return allNodes, nil
}
