CREATE TABLE IF NOT EXISTS clusters (
    id VARCHAR(255) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    region VARCHAR(100) NOT NULL,
    provider_type VARCHAR(50) NOT NULL,
    health VARCHAR(50) NOT NULL DEFAULT 'HEALTHY',
    security_classes JSONB NOT NULL DEFAULT '["standard"]',
    network_cost FLOAT NOT NULL DEFAULT 1.0,
    base_latency_ms FLOAT NOT NULL DEFAULT 10.0,
    labels JSONB NOT NULL DEFAULT '{}',
    total_cpu FLOAT NOT NULL DEFAULT 0.0,
    total_memory FLOAT NOT NULL DEFAULT 0.0,
    available_cpu FLOAT NOT NULL DEFAULT 0.0,
    available_memory FLOAT NOT NULL DEFAULT 0.0,
    last_heartbeat_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_clusters_region ON clusters (region);
CREATE INDEX IF NOT EXISTS idx_clusters_health ON clusters (health);

-- Add fencing_token and execution_generation columns to agent_runs
ALTER TABLE agent_runs ADD COLUMN IF NOT EXISTS fencing_token VARCHAR(255) NOT NULL DEFAULT '';
ALTER TABLE agent_runs ADD COLUMN IF NOT EXISTS execution_generation BIGINT NOT NULL DEFAULT 0;

-- Add fencing_token to run_scheduling_assignments
ALTER TABLE run_scheduling_assignments ADD COLUMN IF NOT EXISTS fencing_token VARCHAR(255) NOT NULL DEFAULT '';
ALTER TABLE run_scheduling_assignments ADD COLUMN IF NOT EXISTS execution_generation BIGINT NOT NULL DEFAULT 0;
