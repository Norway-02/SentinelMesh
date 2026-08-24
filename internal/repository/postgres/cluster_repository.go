package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/sentinelmesh/sentinelmesh/internal/domain"
	"github.com/sentinelmesh/sentinelmesh/internal/repository"
)

type ClusterRepository struct {
	db *sql.DB
}

func NewClusterRepository(db *sql.DB) *ClusterRepository {
	return &ClusterRepository{db: db}
}

func (r *ClusterRepository) Register(ctx context.Context, cluster domain.Cluster) error {
	securityClassesJSON, err := json.Marshal(cluster.SecurityClasses)
	if err != nil {
		return err
	}
	labelsJSON, err := json.Marshal(cluster.Labels)
	if err != nil {
		return err
	}

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

	query := `
		INSERT INTO clusters (
			id, name, region, provider_type, health, security_classes,
			network_cost, base_latency_ms, labels,
			total_cpu, total_memory, available_cpu, available_memory,
			last_heartbeat_at, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9,
			$10, $11, $12, $13,
			$14, $15, $16
		)
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			region = EXCLUDED.region,
			provider_type = EXCLUDED.provider_type,
			health = EXCLUDED.health,
			security_classes = EXCLUDED.security_classes,
			network_cost = EXCLUDED.network_cost,
			base_latency_ms = EXCLUDED.base_latency_ms,
			labels = EXCLUDED.labels,
			total_cpu = EXCLUDED.total_cpu,
			total_memory = EXCLUDED.total_memory,
			available_cpu = EXCLUDED.available_cpu,
			available_memory = EXCLUDED.available_memory,
			last_heartbeat_at = EXCLUDED.last_heartbeat_at,
			updated_at = EXCLUDED.updated_at
	`
	db := extractDB(ctx, r.db)
	_, err = db.ExecContext(ctx, query,
		cluster.ID, cluster.Name, cluster.Region, string(cluster.ProviderType), string(cluster.Status.Health), securityClassesJSON,
		cluster.NetworkCost, cluster.BaseLatencyMs, labelsJSON,
		cluster.Status.TotalCPU, cluster.Status.TotalMemory, cluster.Status.AvailableCPU, cluster.Status.AvailableMemory,
		cluster.Status.LastHeartbeatAt, cluster.CreatedAt, cluster.UpdatedAt,
	)
	return err
}

func (r *ClusterRepository) Get(ctx context.Context, id string) (domain.Cluster, error) {
	query := `
		SELECT 
			id, name, region, provider_type, health, security_classes,
			network_cost, base_latency_ms, labels,
			total_cpu, total_memory, available_cpu, available_memory,
			last_heartbeat_at, created_at, updated_at
		FROM clusters
		WHERE id = $1
	`
	db := extractDB(ctx, r.db)
	row := db.QueryRowContext(ctx, query, id)

	var c domain.Cluster
	var providerType, health string
	var securityClassesJSON, labelsJSON []byte

	err := row.Scan(
		&c.ID, &c.Name, &c.Region, &providerType, &health, &securityClassesJSON,
		&c.NetworkCost, &c.BaseLatencyMs, &labelsJSON,
		&c.Status.TotalCPU, &c.Status.TotalMemory, &c.Status.AvailableCPU, &c.Status.AvailableMemory,
		&c.Status.LastHeartbeatAt, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return domain.Cluster{}, repository.ErrNotFound
		}
		return domain.Cluster{}, err
	}

	c.ProviderType = domain.ClusterProviderType(providerType)
	c.Status.Health = domain.ClusterHealth(health)
	_ = json.Unmarshal(securityClassesJSON, &c.SecurityClasses)
	_ = json.Unmarshal(labelsJSON, &c.Labels)

	return c, nil
}

func (r *ClusterRepository) List(ctx context.Context) ([]domain.Cluster, error) {
	query := `
		SELECT 
			id, name, region, provider_type, health, security_classes,
			network_cost, base_latency_ms, labels,
			total_cpu, total_memory, available_cpu, available_memory,
			last_heartbeat_at, created_at, updated_at
		FROM clusters
		ORDER BY created_at ASC
	`
	db := extractDB(ctx, r.db)
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var clusters []domain.Cluster
	for rows.Next() {
		var c domain.Cluster
		var providerType, health string
		var securityClassesJSON, labelsJSON []byte

		err := rows.Scan(
			&c.ID, &c.Name, &c.Region, &providerType, &health, &securityClassesJSON,
			&c.NetworkCost, &c.BaseLatencyMs, &labelsJSON,
			&c.Status.TotalCPU, &c.Status.TotalMemory, &c.Status.AvailableCPU, &c.Status.AvailableMemory,
			&c.Status.LastHeartbeatAt, &c.CreatedAt, &c.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		c.ProviderType = domain.ClusterProviderType(providerType)
		c.Status.Health = domain.ClusterHealth(health)
		_ = json.Unmarshal(securityClassesJSON, &c.SecurityClasses)
		_ = json.Unmarshal(labelsJSON, &c.Labels)

		clusters = append(clusters, c)
	}

	return clusters, nil
}

func (r *ClusterRepository) UpdateStatus(ctx context.Context, clusterID string, status domain.ClusterStatus) error {
	now := time.Now()
	if status.LastHeartbeatAt.IsZero() {
		status.LastHeartbeatAt = now
	}

	query := `
		UPDATE clusters SET
			health = $1,
			total_cpu = $2,
			total_memory = $3,
			available_cpu = $4,
			available_memory = $5,
			last_heartbeat_at = $6,
			updated_at = $7
		WHERE id = $8
	`
	db := extractDB(ctx, r.db)
	res, err := db.ExecContext(ctx, query,
		string(status.Health), status.TotalCPU, status.TotalMemory, status.AvailableCPU, status.AvailableMemory,
		status.LastHeartbeatAt, now, clusterID,
	)
	if err != nil {
		return err
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return repository.ErrNotFound
	}
	return nil
}

func (r *ClusterRepository) Heartbeat(ctx context.Context, clusterID string) error {
	now := time.Now()
	query := `
		UPDATE clusters SET
			last_heartbeat_at = $1,
			updated_at = $2
		WHERE id = $3
	`
	db := extractDB(ctx, r.db)
	res, err := db.ExecContext(ctx, query, now, now, clusterID)
	if err != nil {
		return err
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return repository.ErrNotFound
	}
	return nil
}

func (r *ClusterRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM clusters WHERE id = $1`
	db := extractDB(ctx, r.db)
	res, err := db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return repository.ErrNotFound
	}
	return nil
}
