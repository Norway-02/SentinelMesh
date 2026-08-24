package memory

import (
	"context"
	"sync"
	"time"

	"github.com/sentinelmesh/sentinelmesh/internal/domain"
	"github.com/sentinelmesh/sentinelmesh/internal/repository"
)

// ClusterRepository provides an in-memory, thread-safe implementation of ClusterRepository.
type ClusterRepository struct {
	mu       sync.RWMutex
	clusters map[string]domain.Cluster
}

// NewClusterRepository creates a new in-memory ClusterRepository.
func NewClusterRepository() *ClusterRepository {
	return &ClusterRepository{
		clusters: make(map[string]domain.Cluster),
	}
}

func (r *ClusterRepository) Register(ctx context.Context, cluster domain.Cluster) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	if cluster.CreatedAt.IsZero() {
		cluster.CreatedAt = now
	}
	cluster.UpdatedAt = now
	if cluster.Status.LastHeartbeatAt.IsZero() {
		cluster.Status.LastHeartbeatAt = now
	}
	if cluster.Status.Health == "" {
		cluster.Status.Health = domain.ClusterHealthHealthy
	}

	r.clusters[cluster.ID] = cluster
	return nil
}

func (r *ClusterRepository) Get(ctx context.Context, id string) (domain.Cluster, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	cluster, exists := r.clusters[id]
	if !exists {
		return domain.Cluster{}, repository.ErrNotFound
	}
	return cluster, nil
}

func (r *ClusterRepository) List(ctx context.Context) ([]domain.Cluster, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	list := make([]domain.Cluster, 0, len(r.clusters))
	for _, c := range r.clusters {
		list = append(list, c)
	}
	return list, nil
}

func (r *ClusterRepository) UpdateStatus(ctx context.Context, clusterID string, status domain.ClusterStatus) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	cluster, exists := r.clusters[clusterID]
	if !exists {
		return repository.ErrNotFound
	}

	now := time.Now()
	if status.LastHeartbeatAt.IsZero() {
		status.LastHeartbeatAt = now
	}
	cluster.Status = status
	cluster.UpdatedAt = now
	r.clusters[clusterID] = cluster
	return nil
}

func (r *ClusterRepository) Heartbeat(ctx context.Context, clusterID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	cluster, exists := r.clusters[clusterID]
	if !exists {
		return repository.ErrNotFound
	}

	now := time.Now()
	cluster.Status.LastHeartbeatAt = now
	cluster.UpdatedAt = now
	r.clusters[clusterID] = cluster
	return nil
}

func (r *ClusterRepository) Delete(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.clusters[id]; !exists {
		return repository.ErrNotFound
	}
	delete(r.clusters, id)
	return nil
}
