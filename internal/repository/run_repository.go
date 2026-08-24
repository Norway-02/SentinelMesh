package repository

import (
	"context"

	"github.com/sentinelmesh/sentinelmesh/internal/domain"
)

type RunRepository interface {
	Create(ctx context.Context, run domain.AgentRun) error
	Get(ctx context.Context, id string) (domain.AgentRun, error)
	Update(ctx context.Context, run domain.AgentRun) error
	ListByNode(ctx context.Context, nodeID string) ([]domain.AgentRun, error)
	ListByCluster(ctx context.Context, clusterID string) ([]domain.AgentRun, error)
}
