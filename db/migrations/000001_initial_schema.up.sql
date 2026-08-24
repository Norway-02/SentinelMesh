CREATE TABLE IF NOT EXISTS agents (
    id VARCHAR(36) PRIMARY KEY,
    tenant_id VARCHAR(255) NOT NULL,
    name VARCHAR(255) NOT NULL,
    version VARCHAR(255) NOT NULL,
    image VARCHAR(255) NOT NULL,
    priority VARCHAR(50) NOT NULL,

    cpu VARCHAR(50) NOT NULL,
    memory VARCHAR(50) NOT NULL,
    gpu INTEGER NOT NULL,

    state VARCHAR(50) NOT NULL,

    security_policy JSONB NOT NULL,
    network_policy JSONB NOT NULL,
    checkpoint_policy JSONB NOT NULL,
    verification_policy JSONB NOT NULL,
    model_policy VARCHAR(255) NOT NULL,

    created_at TIMESTAMP WITH TIME ZONE NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL
);

CREATE INDEX idx_agents_tenant_id ON agents (tenant_id);

CREATE TABLE IF NOT EXISTS agent_runs (
    id VARCHAR(36) PRIMARY KEY,
    agent_id VARCHAR(36) NOT NULL,
    tenant_id VARCHAR(255) NOT NULL,

    state VARCHAR(50) NOT NULL,
    attempt INTEGER NOT NULL DEFAULT 0,
    retry_count INTEGER NOT NULL DEFAULT 0,

    cluster VARCHAR(255) NOT NULL DEFAULT '',
    node VARCHAR(255) NOT NULL DEFAULT '',

    last_checkpoint_id VARCHAR(255) NOT NULL DEFAULT '',

    failure_reason TEXT NOT NULL DEFAULT '',
    verification_state VARCHAR(50) NOT NULL DEFAULT '',

    version BIGINT NOT NULL DEFAULT 1,

    started_at TIMESTAMP WITH TIME ZONE,
    finished_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL,

    CONSTRAINT fk_agent_runs_agent_id FOREIGN KEY (agent_id) REFERENCES agents(id) ON DELETE CASCADE
);

CREATE INDEX idx_agent_runs_agent_id ON agent_runs (agent_id);
CREATE INDEX idx_agent_runs_tenant_id ON agent_runs (tenant_id);
