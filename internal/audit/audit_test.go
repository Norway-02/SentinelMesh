package audit

import (
	"context"
	"testing"
	"time"
)

func TestMemoryRepository_CRUD(t *testing.T) {
	repo := NewMemoryRepository()
	ctx := context.Background()

	now := time.Now()
	event1 := AuditEvent{
		ID:            "audit-1",
		RunID:         "run-123",
		AgentID:       "agent-456",
		TenantID:      "tenant-ops",
		CorrelationID: "corr-1",
		Source:        "policy-engine",
		EventType:     "policy_evaluation",
		Operation:     "file:read",
		Resource:      "/etc/shadow",
		Decision:      "DENY",
		RuleID:        "fs-denied-path",
		Reason:        "forbidden path access",
		Severity:      "HIGH",
		OccurredAt:    now,
	}

	event2 := AuditEvent{
		ID:            "audit-2",
		RunID:         "run-123",
		AgentID:       "agent-456",
		TenantID:      "tenant-ops",
		CorrelationID: "corr-2",
		Source:        "process-enforcer",
		EventType:     "sandbox_violation",
		Operation:     "syscall:exec",
		Resource:      "ptrace",
		Decision:      "DENY",
		RuleID:        "syscall-denied",
		Reason:        "forbidden syscall",
		Severity:      "CRITICAL",
		OccurredAt:    now.Add(1 * time.Second),
	}

	if err := repo.Insert(ctx, event1); err != nil {
		t.Fatalf("Insert failed: %v", err)
	}
	if err := repo.Insert(ctx, event2); err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	// Fetch by RunID
	runs, err := repo.GetByRunID(ctx, "run-123")
	if err != nil {
		t.Fatalf("GetByRunID failed: %v", err)
	}
	if len(runs) != 2 {
		t.Errorf("expected 2 audit events for run-123, got %d", len(runs))
	}

	// Filter by Severity
	crit, err := repo.List(ctx, AuditFilter{Severity: "CRITICAL"})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(crit) != 1 || crit[0].ID != "audit-2" {
		t.Errorf("expected 1 critical audit event, got %v", crit)
	}
}
