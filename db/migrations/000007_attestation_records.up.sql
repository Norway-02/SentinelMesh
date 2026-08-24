-- Migration 000007: Attestation Records and Run Verification State

CREATE TABLE IF NOT EXISTS agent_attestation_records (
    id UUID PRIMARY KEY,
    run_id VARCHAR(64) NOT NULL REFERENCES agent_runs(id) ON DELETE CASCADE,
    agent_id VARCHAR(64) NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    tenant_id VARCHAR(64) NOT NULL,
    status VARCHAR(32) NOT NULL, -- VERIFIED, FAILED
    evidence_digest VARCHAR(128) NOT NULL, -- Canonical SHA-256 hash of all evaluations
    rule_evaluations JSONB NOT NULL,
    attested_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_attestations_run_id ON agent_attestation_records(run_id);
CREATE INDEX IF NOT EXISTS idx_attestations_agent_id ON agent_attestation_records(agent_id);
CREATE INDEX IF NOT EXISTS idx_attestations_status ON agent_attestation_records(status);

ALTER TABLE agent_runs 
ADD COLUMN IF NOT EXISTS attestation_id UUID REFERENCES agent_attestation_records(id);
