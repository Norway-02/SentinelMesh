package repository

import (
	"context"

	"github.com/sentinelmesh/sentinelmesh/internal/domain"
)

// ClusterRepository defines persistence operations for cluster metadata and status.
type ClusterRepository interface {
	Register(ctx context.Context, cluster domain.Cluster) error
	Get(ctx context.Context, id string) (domain.Cluster, error)
	List(ctx context.Context) ([]domain.Cluster, error)
	UpdateStatus(ctx context.Context, clusterID string, status domain.ClusterStatus) error
	Heartbeat(ctx context.Context, clusterID string) error
	Delete(ctx context.Context, id string) error
}
