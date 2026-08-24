CREATE TABLE run_scheduling_assignments (
    run_id VARCHAR(255) PRIMARY KEY,
    cluster_id VARCHAR(255) NOT NULL,
    node_id VARCHAR(255) NOT NULL,
    algorithm_version VARCHAR(100) NOT NULL,
    score FLOAT NOT NULL,
    decision JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    version INTEGER NOT NULL DEFAULT 1
);
