ALTER TABLE agent_runs ADD COLUMN IF NOT EXISTS recovered_from_checkpoint_id VARCHAR(255) NOT NULL DEFAULT '';
ALTER TABLE agent_runs ADD COLUMN IF NOT EXISTS recovery_generation INT NOT NULL DEFAULT 0;

CREATE TABLE IF NOT EXISTS agent_checkpoints (
    id VARCHAR(64) PRIMARY KEY,
    run_id VARCHAR(64) NOT NULL REFERENCES agent_runs(id) ON DELETE CASCADE,
    agent_id VARCHAR(64) NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    tenant_id VARCHAR(255) NOT NULL,
    sequence_number BIGINT NOT NULL,
    state_inline JSONB,
    state_uri TEXT,
    state_checksum VARCHAR(64) NOT NULL,
    size_bytes BIGINT NOT NULL,
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_agent_checkpoints_run_seq UNIQUE (run_id, sequence_number)
);

CREATE INDEX IF NOT EXISTS idx_agent_checkpoints_run_seq_desc ON agent_checkpoints(run_id, sequence_number DESC);
CREATE INDEX IF NOT EXISTS idx_agent_checkpoints_tenant_id ON agent_checkpoints(tenant_id);
CREATE INDEX IF NOT EXISTS idx_agent_checkpoints_created_at ON agent_checkpoints(created_at);
