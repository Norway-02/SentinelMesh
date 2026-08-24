CREATE TABLE outbox_events (
    id UUID PRIMARY KEY,
    aggregate_type VARCHAR(100) NOT NULL,
    aggregate_id VARCHAR(255) NOT NULL,
    event_type VARCHAR(100) NOT NULL,
    schema_version INTEGER NOT NULL,
    tenant_id VARCHAR(255) NOT NULL,
    correlation_id VARCHAR(255),
    payload JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    claimed_at TIMESTAMPTZ,
    claim_owner VARCHAR(255),

    published_at TIMESTAMPTZ,

    attempts INTEGER NOT NULL DEFAULT 0,
    last_error TEXT
);

CREATE INDEX idx_outbox_pending
ON outbox_events (created_at)
WHERE published_at IS NULL;

CREATE INDEX idx_outbox_claimed
ON outbox_events (claimed_at)
WHERE published_at IS NULL;
