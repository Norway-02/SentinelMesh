CREATE TABLE IF NOT EXISTS security_audit_events (
    id VARCHAR(64) PRIMARY KEY,
    run_id VARCHAR(64) NOT NULL,
    agent_id VARCHAR(64) NOT NULL,
    tenant_id VARCHAR(64) NOT NULL,
    correlation_id VARCHAR(64) NOT NULL,
    source VARCHAR(64) NOT NULL,
    event_type VARCHAR(64) NOT NULL,
    operation VARCHAR(64) NOT NULL,
    resource TEXT NOT NULL,
    decision VARCHAR(32) NOT NULL,
    rule_id VARCHAR(64) NOT NULL,
    reason TEXT NOT NULL,
    severity VARCHAR(32) NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    metadata JSONB
);

CREATE INDEX IF NOT EXISTS idx_security_audit_run_id ON security_audit_events(run_id);
CREATE INDEX IF NOT EXISTS idx_security_audit_tenant_id ON security_audit_events(tenant_id);
CREATE INDEX IF NOT EXISTS idx_security_audit_occurred_at ON security_audit_events(occurred_at);
