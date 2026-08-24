package audit

import (
	"context"
	"time"
)

// AuditEvent records a durable forensic record of a policy decision or security enforcement action.
type AuditEvent struct {
	ID            string                 `json:"id"`
	RunID         string                 `json:"run_id"`
	AgentID       string                 `json:"agent_id"`
	TenantID      string                 `json:"tenant_id"`
	CorrelationID string                 `json:"correlation_id"`
	Source        string                 `json:"source"` // "policy-engine", "process-enforcer", "kubernetes-enforcer", "runtime"
	EventType     string                 `json:"event_type"`
	Operation     string                 `json:"operation"`
	Resource      string                 `json:"resource"`
	Decision      string                 `json:"decision"`
	RuleID        string                 `json:"rule_id"`
	Reason        string                 `json:"reason"`
	Severity      string                 `json:"severity"`
	OccurredAt    time.Time              `json:"occurred_at"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// AuditFilter allows querying audit records with specific criteria.
type AuditFilter struct {
	RunID    string
	AgentID  string
	TenantID string
	Decision string
	Severity string
	Limit    int
}

// Repository defines persistence operations for security audit trails.
type Repository interface {
	Insert(ctx context.Context, event AuditEvent) error
	GetByRunID(ctx context.Context, runID string) ([]AuditEvent, error)
	List(ctx context.Context, filter AuditFilter) ([]AuditEvent, error)
}
