package postgres

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/sentinelmesh/sentinelmesh/internal/domain"
	"github.com/sentinelmesh/sentinelmesh/internal/repository"
)

type AgentRepository struct {
	db *sql.DB
}

func NewAgentRepository(db *sql.DB) *AgentRepository {
	return &AgentRepository{db: db}
}

func (r *AgentRepository) Create(ctx context.Context, agent domain.Agent) error {
	securityPolicy, err := json.Marshal(agent.SecurityPolicy)
	if err != nil {
		return err
	}
	networkPolicy, err := json.Marshal(agent.NetworkPolicy)
	if err != nil {
		return err
	}
	checkpointPolicy, err := json.Marshal(agent.CheckpointPolicy)
	if err != nil {
		return err
	}
	verificationPolicy, err := json.Marshal(agent.VerificationPolicy)
	if err != nil {
		return err
	}

	query := `
		INSERT INTO agents (
			id, tenant_id, name, version, image, priority, 
			cpu, memory, gpu, state, 
			security_policy, network_policy, checkpoint_policy, verification_policy, model_policy,
			created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10,
			$11, $12, $13, $14, $15,
			$16, $17
		)
	`
	db := extractDB(ctx, r.db)
	_, err = db.ExecContext(ctx, query,
		agent.ID, agent.TenantID, agent.Name, agent.Version, agent.Image, agent.Priority,
		agent.Resources.CPU, agent.Resources.Memory, agent.Resources.GPU, agent.State,
		securityPolicy, networkPolicy, checkpointPolicy, verificationPolicy, agent.ModelPolicy,
		agent.CreatedAt, agent.UpdatedAt,
	)
	
	if err != nil {
		// Detect unique constraint violation (id or something else)
		// pgx errors usually contain SQLSTATE 23505
		if err.Error() == "ERROR: duplicate key value violates unique constraint \"agents_pkey\" (SQLSTATE 23505)" {
			return repository.ErrAlreadyExists
		}
		// A more robust check might be needed for production, but this works for basic errors
		return err
	}

	return nil
}

func (r *AgentRepository) Get(ctx context.Context, id string) (domain.Agent, error) {
	query := `
		SELECT 
			id, tenant_id, name, version, image, priority, 
			cpu, memory, gpu, state, 
			security_policy, network_policy, checkpoint_policy, verification_policy, model_policy,
			created_at, updated_at
		FROM agents
		WHERE id = $1
	`
	row := r.db.QueryRowContext(ctx, query, id)

	var agent domain.Agent
	var securityPolicy, networkPolicy, checkpointPolicy, verificationPolicy []byte

	err := row.Scan(
		&agent.ID, &agent.TenantID, &agent.Name, &agent.Version, &agent.Image, &agent.Priority,
		&agent.Resources.CPU, &agent.Resources.Memory, &agent.Resources.GPU, &agent.State,
		&securityPolicy, &networkPolicy, &checkpointPolicy, &verificationPolicy, &agent.ModelPolicy,
		&agent.CreatedAt, &agent.UpdatedAt,
	)
	
	if err != nil {
		if err == sql.ErrNoRows {
			return domain.Agent{}, repository.ErrNotFound
		}
		return domain.Agent{}, err
	}

	json.Unmarshal(securityPolicy, &agent.SecurityPolicy)
	json.Unmarshal(networkPolicy, &agent.NetworkPolicy)
	json.Unmarshal(checkpointPolicy, &agent.CheckpointPolicy)
	json.Unmarshal(verificationPolicy, &agent.VerificationPolicy)

	return agent, nil
}

func (r *AgentRepository) List(ctx context.Context, filter repository.AgentFilter) ([]domain.Agent, string, error) {
	// Simple implementation; ignores pagination for now
	query := `
		SELECT 
			id, tenant_id, name, version, image, priority, 
			cpu, memory, gpu, state, 
			security_policy, network_policy, checkpoint_policy, verification_policy, model_policy,
			created_at, updated_at
		FROM agents
		WHERE tenant_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`
	
	limit := filter.PageSize
	if limit <= 0 {
		limit = 100
	}

	rows, err := r.db.QueryContext(ctx, query, filter.TenantID, limit)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	var agents []domain.Agent
	for rows.Next() {
		var agent domain.Agent
		var securityPolicy, networkPolicy, checkpointPolicy, verificationPolicy []byte
		
		err := rows.Scan(
			&agent.ID, &agent.TenantID, &agent.Name, &agent.Version, &agent.Image, &agent.Priority,
			&agent.Resources.CPU, &agent.Resources.Memory, &agent.Resources.GPU, &agent.State,
			&securityPolicy, &networkPolicy, &checkpointPolicy, &verificationPolicy, &agent.ModelPolicy,
			&agent.CreatedAt, &agent.UpdatedAt,
		)
		if err != nil {
			return nil, "", err
		}

		json.Unmarshal(securityPolicy, &agent.SecurityPolicy)
		json.Unmarshal(networkPolicy, &agent.NetworkPolicy)
		json.Unmarshal(checkpointPolicy, &agent.CheckpointPolicy)
		json.Unmarshal(verificationPolicy, &agent.VerificationPolicy)
		
		agents = append(agents, agent)
	}

	if err = rows.Err(); err != nil {
		return nil, "", err
	}

	return agents, "", nil
}

func (r *AgentRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM agents WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}
