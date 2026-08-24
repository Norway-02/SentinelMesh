CREATE TABLE IF NOT EXISTS agent_run_state_history (
    id VARCHAR(36) PRIMARY KEY,
    run_id VARCHAR(36) NOT NULL,
    from_state VARCHAR(50) NOT NULL,
    to_state VARCHAR(50) NOT NULL,
    reason TEXT NOT NULL DEFAULT '',
    timestamp TIMESTAMP WITH TIME ZONE NOT NULL,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,

    CONSTRAINT fk_state_history_run_id FOREIGN KEY (run_id) REFERENCES agent_runs(id) ON DELETE CASCADE
);

CREATE INDEX idx_state_history_run_id ON agent_run_state_history (run_id);
