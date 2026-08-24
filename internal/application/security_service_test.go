package application

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/sentinelmesh/sentinelmesh/internal/audit"
	"github.com/sentinelmesh/sentinelmesh/internal/events"
	"github.com/sentinelmesh/sentinelmesh/internal/outbox"
	"github.com/sentinelmesh/sentinelmesh/internal/policy"
	"github.com/sentinelmesh/sentinelmesh/internal/repository/memory"
)

func TestSecurityService_AllowedOperation_AuditsWithoutViolationEvent(t *testing.T) {
	engine := policy.NewEngine()
	auditRepo := audit.NewMemoryRepository()
	outboxRepo := outbox.NewMemoryRepository()
	txManager := memory.NewTxManager()

	svc := NewSecurityService(engine, auditRepo, outboxRepo, txManager)
	ctx := context.Background()

	req := policy.EvaluationRequest{
		RunID:         "run-allow-01",
		AgentID:       "agent-01",
		TenantID:      "tenant-prod",
		CorrelationID: "corr-12345",
		Profile:       policy.ProfileStandard,
		Operation:     "file:read",
		Resource:      "/workspace/data.json",
	}

	res, err := svc.EvaluateAndEnforce(ctx, req, "policy-engine")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Decision != policy.DecisionAllow {
		t.Errorf("expected ALLOW, got %s", res.Decision)
	}

	// 1. Verify audit log was stored
	records, err := auditRepo.GetByRunID(ctx, "run-allow-01")
	if err != nil {
		t.Fatalf("failed to fetch audit records: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 audit record, got %d", len(records))
	}
	if records[0].CorrelationID != "corr-12345" {
		t.Errorf("expected correlation ID 'corr-12345', got '%s'", records[0].CorrelationID)
	}
	if records[0].Decision != "ALLOW" {
		t.Errorf("expected decision ALLOW, got %s", records[0].Decision)
	}

	// 2. Verify NO violation event was emitted to outbox
	pending := outboxRepo.GetEvents()
	if len(pending) != 0 {
		t.Errorf("expected 0 outbox events for allowed operation, got %d", len(pending))
	}
}

func TestSecurityService_Violation_AuditsAndPublishesOutbox(t *testing.T) {
	engine := policy.NewEngine()
	auditRepo := audit.NewMemoryRepository()
	outboxRepo := outbox.NewMemoryRepository()
	txManager := memory.NewTxManager()

	svc := NewSecurityService(engine, auditRepo, outboxRepo, txManager)
	ctx := context.Background()

	req := policy.EvaluationRequest{
		RunID:         "run-deny-01",
		AgentID:       "agent-02",
		TenantID:      "tenant-prod",
		CorrelationID: "corr-violation-99",
		Profile:       policy.ProfileStandard,
		Operation:     "file:read",
		Resource:      "/workspace/../etc/shadow",
	}

	res, err := svc.EvaluateAndEnforce(ctx, req, "policy-engine")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Decision != policy.DecisionDeny {
		t.Errorf("expected DENY, got %s", res.Decision)
	}

	// 1. Verify audit record
	records, err := auditRepo.GetByRunID(ctx, "run-deny-01")
	if err != nil {
		t.Fatalf("failed to fetch audit records: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 audit record, got %d", len(records))
	}
	if records[0].CorrelationID != "corr-violation-99" {
		t.Errorf("expected correlation ID 'corr-violation-99', got '%s'", records[0].CorrelationID)
	}
	if records[0].Decision != "DENY" {
		t.Errorf("expected decision DENY, got %s", records[0].Decision)
	}
	if records[0].RuleID != "fs-denied-path" && records[0].RuleID != "fs-default-deny" {
		t.Errorf("expected fs deny rule ID, got %s", records[0].RuleID)
	}

	// 2. Verify outbox event emitted
	pendingEvents := outboxRepo.GetEvents()
	if len(pendingEvents) != 1 {
		t.Fatalf("expected 1 outbox event for violation, got %d", len(pendingEvents))
	}

	outboxItem := pendingEvents[0]
	if outboxItem.EventType != events.SubjectSecurityPolicyViolation {
		t.Errorf("expected event type %s, got %s", events.SubjectSecurityPolicyViolation, outboxItem.EventType)
	}

	var payload events.SecurityViolationPayload
	if err := json.Unmarshal(outboxItem.Payload, &payload); err != nil {
		t.Fatalf("failed to unmarshal payload: %v", err)
	}

	if payload.CorrelationID != "corr-violation-99" {
		t.Errorf("expected payload correlation ID 'corr-violation-99', got '%s'", payload.CorrelationID)
	}
	if payload.Decision != "DENY" {
		t.Errorf("expected payload decision DENY, got %s", payload.Decision)
	}
	if payload.Source != "policy-engine" {
		t.Errorf("expected payload source 'policy-engine', got %s", payload.Source)
	}
	if payload.EventID == "" {
		t.Errorf("expected non-empty EventID in payload")
	}
}
