package application

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/sentinelmesh/sentinelmesh/internal/audit"
	"github.com/sentinelmesh/sentinelmesh/internal/events"
	"github.com/sentinelmesh/sentinelmesh/internal/outbox"
	"github.com/sentinelmesh/sentinelmesh/internal/policy"
	"github.com/sentinelmesh/sentinelmesh/internal/repository"
)

// SecurityService coordinates pure policy evaluation with audit logging and outbox event dispatching.
type SecurityService struct {
	engine     *policy.Engine
	auditRepo  audit.Repository
	outboxRepo outbox.Repository
	txManager  repository.TxManager
}

// NewSecurityService constructs a SecurityService instance.
func NewSecurityService(
	engine *policy.Engine,
	auditRepo audit.Repository,
	outboxRepo outbox.Repository,
	txManager repository.TxManager,
) *SecurityService {
	return &SecurityService{
		engine:     engine,
		auditRepo:  auditRepo,
		outboxRepo: outboxRepo,
		txManager:  txManager,
	}
}

// Engine returns the underlying policy engine.
func (s *SecurityService) Engine() *policy.Engine {
	return s.engine
}

// EvaluateAndEnforce evaluates an intent request, persists audit events, and emits outbox events on violation.
func (s *SecurityService) EvaluateAndEnforce(ctx context.Context, req policy.EvaluationRequest, source string) (policy.EvaluationResult, error) {
	if source == "" {
		source = "policy-engine"
	}

	result := s.engine.Evaluate(req)

	// Create audit record
	auditID := uuid.NewString()
	auditEvt := audit.AuditEvent{
		ID:            auditID,
		RunID:         req.RunID,
		AgentID:       req.AgentID,
		TenantID:      req.TenantID,
		CorrelationID: req.CorrelationID,
		Source:        source,
		EventType:     "policy_evaluation",
		Operation:     req.Operation,
		Resource:      req.Resource,
		Decision:      string(result.Decision),
		RuleID:        result.RuleID,
		Reason:        result.Reason,
		Severity:      string(result.Severity),
		OccurredAt:    result.Timestamp,
		Metadata: map[string]interface{}{
			"profile":  string(req.Profile),
			"duration": result.Duration.String(),
		},
	}

	// Always insert audit record
	if s.auditRepo != nil {
		if err := s.auditRepo.Insert(ctx, auditEvt); err != nil {
			return result, fmt.Errorf("failed to persist security audit event: %w", err)
		}
	}

	// If decision is DENY or REQUIRE_APPROVAL, publish violation to outbox
	if result.Decision == policy.DecisionDeny || result.Decision == policy.DecisionRequireApproval {
		if s.outboxRepo != nil {
			payload, _ := json.Marshal(events.SecurityViolationPayload{
				EventID:       auditID,
				RunID:         req.RunID,
				AgentID:       req.AgentID,
				TenantID:      req.TenantID,
				CorrelationID: req.CorrelationID,
				Source:        source,
				Operation:     req.Operation,
				Resource:      req.Resource,
				RuleID:        result.RuleID,
				Decision:      string(result.Decision),
				Severity:      string(result.Severity),
				Reason:        result.Reason,
				OccurredAt:    result.Timestamp,
			})

			subject := events.SubjectSecurityPolicyViolation
			if source == "process-enforcer" || source == "kubernetes-enforcer" || source == "runtime" {
				subject = events.SubjectSecuritySandboxViolation
			}

			outboxEvt := events.Event{
				EventID:       uuid.NewString(),
				EventType:     subject,
				SchemaVersion: 1,
				AggregateType: "Security",
				AggregateID:   req.RunID,
				TenantID:      req.TenantID,
				CorrelationID: req.CorrelationID,
				OccurredAt:    time.Now(),
				Payload:       payload,
			}

			if err := s.outboxRepo.Insert(ctx, outboxEvt); err != nil {
				return result, fmt.Errorf("failed to enqueue security violation event: %w", err)
			}
		}
	}

	return result, nil
}
