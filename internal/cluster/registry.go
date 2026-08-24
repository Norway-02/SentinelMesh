package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/sentinelmesh/sentinelmesh/internal/domain"
	"github.com/sentinelmesh/sentinelmesh/internal/events"
	"github.com/sentinelmesh/sentinelmesh/internal/outbox"
	"github.com/sentinelmesh/sentinelmesh/internal/repository"
)

// ClusterRegistry manages cluster registrations, heartbeats, and live health status.
type ClusterRegistry struct {
	mu         sync.RWMutex
	repo       repository.ClusterRepository
	outboxRepo outbox.Repository
}

// NewClusterRegistry creates a new ClusterRegistry instance.
func NewClusterRegistry(repo repository.ClusterRepository, outboxRepo outbox.Repository) *ClusterRegistry {
	return &ClusterRegistry{
		repo:       repo,
		outboxRepo: outboxRepo,
	}
}

// Register registers or updates cluster metadata and initial status.
func (r *ClusterRegistry) Register(ctx context.Context, c domain.Cluster) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.repo.Register(ctx, c)
}

// Get retrieves cluster by ID.
func (r *ClusterRegistry) Get(ctx context.Context, id string) (domain.Cluster, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.repo.Get(ctx, id)
}

// List retrieves all registered clusters.
func (r *ClusterRegistry) List(ctx context.Context) ([]domain.Cluster, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.repo.List(ctx)
}

// ListAvailableClusters returns all clusters in HEALTHY or DEGRADED state.
func (r *ClusterRegistry) ListAvailableClusters(ctx context.Context) ([]domain.Cluster, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	all, err := r.repo.List(ctx)
	if err != nil {
		return nil, err
	}

	var available []domain.Cluster
	for _, c := range all {
		if c.Status.Health.IsAvailable() {
			available = append(available, c)
		}
	}
	return available, nil
}

// Heartbeat processes a periodic heartbeat from a remote cluster operator/agent.
func (r *ClusterRegistry) Heartbeat(ctx context.Context, clusterID string, status domain.ClusterStatus) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	existing, err := r.repo.Get(ctx, clusterID)
	if err != nil {
		return err
	}

	status.LastHeartbeatAt = time.Now()
	if status.Health == "" {
		status.Health = domain.ClusterHealthHealthy
	}

	if err := r.repo.UpdateStatus(ctx, clusterID, status); err != nil {
		return err
	}

	// Emit heartbeat event
	if r.outboxRepo != nil {
		payload, _ := json.Marshal(events.ClusterHeartbeatPayload{
			ClusterID:       clusterID,
			Health:          string(status.Health),
			TotalCPU:        status.TotalCPU,
			TotalMemory:     status.TotalMemory,
			AvailableCPU:    status.AvailableCPU,
			AvailableMemory: status.AvailableMemory,
			Timestamp:       status.LastHeartbeatAt,
		})

		_ = r.outboxRepo.Insert(ctx, events.Event{
			EventID:       uuid.NewString(),
			EventType:     events.SubjectClusterHeartbeat,
			SchemaVersion: 1,
			AggregateType: "Cluster",
			AggregateID:   clusterID,
			TenantID:      "system",
			OccurredAt:    status.LastHeartbeatAt,
			Payload:       payload,
		})
	}

	_ = existing
	return nil
}

// CheckHeartbeatLeases inspects all clusters and flags clusters whose heartbeat has expired as UNREACHABLE.
func (r *ClusterRegistry) CheckHeartbeatLeases(ctx context.Context, timeout time.Duration) ([]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	clusters, err := r.repo.List(ctx)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	var unreachableClusterIDs []string

	for _, c := range clusters {
		if c.Status.Health == domain.ClusterHealthDrained {
			continue
		}

		if now.Sub(c.Status.LastHeartbeatAt) > timeout {
			if c.Status.Health != domain.ClusterHealthUnreachable {
				c.Status.Health = domain.ClusterHealthUnreachable
				_ = r.repo.UpdateStatus(ctx, c.ID, c.Status)
				unreachableClusterIDs = append(unreachableClusterIDs, c.ID)

				// Emit ClusterUnreachable event
				if r.outboxRepo != nil {
					payload, _ := json.Marshal(events.ClusterUnreachablePayload{
						ClusterID:     c.ID,
						Region:        c.Region,
						Reason:        fmt.Sprintf("heartbeat lease expired (> %v)", timeout),
						UnreachableAt: now,
					})

					_ = r.outboxRepo.Insert(ctx, events.Event{
						EventID:       uuid.NewString(),
						EventType:     events.SubjectClusterUnreachable,
						SchemaVersion: 1,
						AggregateType: "Cluster",
						AggregateID:   c.ID,
						TenantID:      "system",
						OccurredAt:    now,
						Payload:       payload,
					})
				}
			}
		}
	}

	return unreachableClusterIDs, nil
}
