package verification

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/sentinelmesh/sentinelmesh/internal/events"
	"github.com/sentinelmesh/sentinelmesh/internal/observability"
	"github.com/sentinelmesh/sentinelmesh/internal/outbox"
	"github.com/sentinelmesh/sentinelmesh/internal/repository"
	"github.com/sentinelmesh/sentinelmesh/internal/types"
)

// VerifyRunRequest encapsulates input for outcome attestation.
type VerifyRunRequest struct {
	RunID           string            `json:"run_id"`
	ReportedMetrics map[string]string `json:"reported_metrics,omitempty"`
}

// Service orchestrates the verification rule engine and cryptographic attestation pipeline.
type Service struct {
	attestationRepo Repository
	agentRepo       repository.AgentRepository
	runRepo         repository.RunRepository
	outboxRepo      outbox.Repository
	txManager       repository.TxManager
	k8sClient       client.Client
	httpClient      *http.Client
}

// NewService constructs a verification Service.
func NewService(
	attestationRepo Repository,
	agentRepo repository.AgentRepository,
	runRepo repository.RunRepository,
	outboxRepo outbox.Repository,
	txManager repository.TxManager,
	k8sClient client.Client,
	httpClient *http.Client,
) *Service {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 5 * time.Second}
	}
	return &Service{
		attestationRepo: attestationRepo,
		agentRepo:       agentRepo,
		runRepo:         runRepo,
		outboxRepo:      outboxRepo,
		txManager:       txManager,
		k8sClient:       k8sClient,
		httpClient:      httpClient,
	}
}

// VerifyRun executes independent evaluation of reality and persists attestation evidence.
func (s *Service) VerifyRun(ctx context.Context, req VerifyRunRequest) (*AttestationRecord, error) {
	ctx, span := observability.StartSpan(ctx, "verification.verify_run")
	defer span.End()

	m := observability.GetMetrics()
	startTime := time.Now()

	observability.InjectSpanAttributes(span, req.RunID, "", "", observability.GetCorrelationID(ctx), 0)

	run, err := s.runRepo.Get(ctx, req.RunID)
	if err != nil {
		observability.RecordSpanError(span, err)
		return nil, fmt.Errorf("failed to fetch run %s: %w", req.RunID, err)
	}

	agent, err := s.agentRepo.Get(ctx, run.AgentID)
	if err != nil {
		observability.RecordSpanError(span, err)
		return nil, fmt.Errorf("failed to fetch agent %s: %w", run.AgentID, err)
	}

	observability.InjectSpanAttributes(span, run.ID, run.AgentID, run.TenantID, observability.GetCorrelationID(ctx), run.RecoveryGeneration)

	// Transition run to VERIFYING
	_ = run.TransitionTo(types.StateVerifying)
	run.VerificationState = "VERIFYING"
	_ = s.runRepo.Update(ctx, run)
	// Refresh run to obtain incremented version
	run, _ = s.runRepo.Get(ctx, req.RunID)

	now := time.Now()
	var evaluations []RuleEvaluation

	// 1. Evaluate Artifact Rules
	for _, rule := range agent.VerificationPolicy.ArtifactRules {
		eval := EvaluateArtifactRule(ctx, rule)
		evaluations = append(evaluations, eval)
	}

	// 2. Evaluate Kubernetes State Rules
	for _, rule := range agent.VerificationPolicy.KubernetesRules {
		eval := EvaluateKubernetesStateRule(ctx, s.k8sClient, rule)
		evaluations = append(evaluations, eval)
	}

	// 3. Evaluate HTTP Health Rules
	for _, rule := range agent.VerificationPolicy.HTTPRules {
		eval := EvaluateHTTPHealthRule(ctx, s.httpClient, rule)
		evaluations = append(evaluations, eval)
	}

	// 4. Evaluate Invariant Rules
	for _, rule := range agent.VerificationPolicy.InvariantRules {
		eval := EvaluateInvariantRule(req.ReportedMetrics, rule)
		evaluations = append(evaluations, eval)
	}

	// 5. Evaluate Command Rules
	for _, rule := range agent.VerificationPolicy.CommandRules {
		eval := EvaluateCommandRule(ctx, rule)
		evaluations = append(evaluations, eval)
	}

	// Determine overall status
	status := StatusVerified
	var failedRuleID, failReason string
	for _, eval := range evaluations {
		if eval.Status == RuleFail || eval.Status == RuleError {
			status = StatusFailed
			failedRuleID = eval.RuleID
			failReason = eval.Reason
			break
		}
	}

	// If verification was disabled and no rules failed, mark verified
	if !agent.VerificationPolicy.Enabled && len(evaluations) == 0 {
		evaluations = append(evaluations, RuleEvaluation{
			RuleID:   "verification-disabled-default-pass",
			RuleType: "system",
			Status:   RulePass,
			Reason:   "verification policy disabled for agent",
		})
	}

	evidenceDigest := ComputeEvidenceDigest(evaluations)
	attestationID := uuid.NewString()

	attestation := AttestationRecord{
		ID:             attestationID,
		RunID:          run.ID,
		AgentID:        run.AgentID,
		TenantID:       run.TenantID,
		Status:         status,
		EvidenceDigest: evidenceDigest,
		Evaluations:    evaluations,
		AttestedAt:     now,
		CreatedAt:      now,
	}

	traceID, _ := observability.GetTraceAndSpanID(ctx)
	corrID := observability.GetCorrelationID(ctx)

	// Atomic persistence and run state update
	persistOp := func(txCtx context.Context) error {
		if err := s.attestationRepo.Save(txCtx, attestation); err != nil {
			return fmt.Errorf("failed to save attestation record: %w", err)
		}

		run.AttestationID = attestationID
		if status == StatusVerified {
			run.VerificationState = "VERIFIED"
			_ = run.TransitionTo(types.StateCompleted)
		} else {
			run.VerificationState = "FAILED"
			run.FailureReason = fmt.Sprintf("verification failed: rule %s (%s)", failedRuleID, failReason)
			_ = run.TransitionTo(types.StateFailed)
		}

		if err := s.runRepo.Update(txCtx, run); err != nil {
			return fmt.Errorf("failed to update run state: %w", err)
		}

		// Outbox event emission
		if s.outboxRepo != nil {
			var eventType string
			var payloadBytes []byte

			if status == StatusVerified {
				eventType = events.SubjectRunVerified
				payloadBytes, _ = json.Marshal(events.RunVerifiedPayload{
					RunID:            run.ID,
					AgentID:          run.AgentID,
					TenantID:         run.TenantID,
					AttestationID:    attestationID,
					EvidenceDigest:   evidenceDigest,
					RulesPassedCount: len(evaluations),
					VerifiedAt:       now,
				})
			} else {
				eventType = events.SubjectRunVerificationFailed
				payloadBytes, _ = json.Marshal(events.RunVerificationFailedPayload{
					RunID:          run.ID,
					AgentID:        run.AgentID,
					TenantID:       run.TenantID,
					AttestationID:  attestationID,
					EvidenceDigest: evidenceDigest,
					FailedRuleID:   failedRuleID,
					Reason:         failReason,
					FailedAt:       now,
				})
			}

			_ = s.outboxRepo.Insert(txCtx, events.Event{
				EventID:       uuid.NewString(),
				EventType:     eventType,
				SchemaVersion: 1,
				AggregateType: "Run",
				AggregateID:   run.ID,
				TenantID:      run.TenantID,
				CorrelationID: corrID,
				TraceParent:   traceID,
				OccurredAt:    now,
				Payload:       payloadBytes,
			})
		}

		return nil
	}

	if s.txManager != nil {
		if err := s.txManager.WithinTx(ctx, persistOp); err != nil {
			observability.RecordSpanError(span, err)
			return nil, err
		}
	} else {
		if err := persistOp(ctx); err != nil {
			observability.RecordSpanError(span, err)
			return nil, err
		}
	}

	duration := time.Since(startTime).Seconds()
	m.VerificationTotal.WithLabelValues("policy_rules").Inc()
	m.VerificationDurationSec.WithLabelValues("policy_rules").Observe(duration)

	span.SetAttributes(
		attribute.String("verification.attestation_id", attestationID),
		attribute.String("verification.status", string(status)),
		attribute.String("verification.evidence_digest", evidenceDigest),
		attribute.Int("verification.rules_count", len(evaluations)),
	)

	if status == StatusVerified {
		m.VerificationSuccessTotal.WithLabelValues("policy_rules").Inc()
		m.RunsCompletedTotal.WithLabelValues("VERIFIED").Inc()
	} else {
		m.VerificationFailureTotal.WithLabelValues("policy_rules", failedRuleID).Inc()
		m.RunsFailedTotal.WithLabelValues("verification_failure").Inc()
	}

	if status == StatusFailed {
		err := fmt.Errorf("%w: rule %s failed (%s)", ErrVerificationFailed, failedRuleID, failReason)
		observability.RecordSpanError(span, err)
		return &attestation, err
	}

	return &attestation, nil
}
