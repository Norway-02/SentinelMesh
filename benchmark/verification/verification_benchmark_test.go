package verification_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/sentinelmesh/sentinelmesh/internal/domain"
	"github.com/sentinelmesh/sentinelmesh/internal/outbox"
	"github.com/sentinelmesh/sentinelmesh/internal/repository/memory"
	"github.com/sentinelmesh/sentinelmesh/internal/types"
	"github.com/sentinelmesh/sentinelmesh/internal/verification"
)

func BenchmarkVerification_AttestationDigest(b *testing.B) {
	ruleCounts := []int{5, 20, 50, 100}

	for _, count := range ruleCounts {
		evals := make([]verification.RuleEvaluation, count)
		for i := 0; i < count; i++ {
			evals[i] = verification.RuleEvaluation{
				RuleID:         fmt.Sprintf("rule-invariant-%03d", i),
				RuleType:       "invariant",
				Status:         verification.RulePass,
				Reason:         "assertion passed",
				EvaluatedValue: "200",
				ExpectedValue:  "200",
				DurationNs:     1500,
			}
		}

		b.Run(fmt.Sprintf("Rules_%d", count), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				digest := verification.ComputeEvidenceDigest(evals)
				if digest == "" {
					b.Fatal("Empty digest")
				}
			}
		})
	}
}

func BenchmarkVerification_ServiceEvaluation(b *testing.B) {
	ctx := context.Background()
	attestRepo := verification.NewMemoryRepository()
	outboxRepo := outbox.NewMemoryRepository()
	runRepo := memory.NewRunRepository()
	agentRepo := memory.NewAgentRepository()
	txManager := memory.NewTxManager()

	agent := domain.Agent{
		ID:             "agent-verify-bench",
		TenantID:       "tenant-1",
		Name:           "verify-agent",
		Version:        "1.0.0",
		Resources:      types.AgentResources{CPU: "1", Memory: "1024Mi"},
		SecurityPolicy: types.SecurityPolicy{Profile: "standard"},
		VerificationPolicy: types.VerificationPolicy{
			Enabled: true,
			InvariantRules: []types.InvariantRule{
				{
					ID:            "rule-1",
					MetricName:    "status",
					Operator:      "eq",
					ExpectedValue: "success",
				},
				{
					ID:            "rule-2",
					MetricName:    "exit_code",
					Operator:      "eq",
					ExpectedValue: "0",
				},
			},
		},
		Image: "sentinelmesh/worker:v1",
	}
	_ = agentRepo.Create(ctx, agent)

	svc := verification.NewService(attestRepo, agentRepo, runRepo, outboxRepo, txManager, nil, nil)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		runID := fmt.Sprintf("run-verify-%d", i)
		_ = runRepo.Create(ctx, domain.AgentRun{
			ID:        runID,
			AgentID:   agent.ID,
			TenantID:  agent.TenantID,
			State:     types.StateRunning,
			Version:   1,
			CreatedAt: time.Now(),
		})

		req := verification.VerifyRunRequest{
			RunID: runID,
			ReportedMetrics: map[string]string{
				"status":    "success",
				"exit_code": "0",
			},
		}

		attestation, err := svc.VerifyRun(ctx, req)
		if err != nil {
			b.Fatalf("VerifyRun failed: %v", err)
		}
		if !attestation.VerifyDigest() {
			b.Fatal("Attestation digest validation failed")
		}
	}
}
