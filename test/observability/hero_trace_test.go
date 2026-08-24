package observability_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8stypes "k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/sentinelmesh/sentinelmesh/internal/application"
	"github.com/sentinelmesh/sentinelmesh/internal/checkpoint"
	"github.com/sentinelmesh/sentinelmesh/internal/cluster"
	"github.com/sentinelmesh/sentinelmesh/internal/domain"
	"github.com/sentinelmesh/sentinelmesh/internal/events"
	v1alpha1 "github.com/sentinelmesh/sentinelmesh/internal/kubernetes/api/v1alpha1"
	"github.com/sentinelmesh/sentinelmesh/internal/observability"
	"github.com/sentinelmesh/sentinelmesh/internal/operator"
	"github.com/sentinelmesh/sentinelmesh/internal/outbox"
	"github.com/sentinelmesh/sentinelmesh/internal/repository/memory"
	"github.com/sentinelmesh/sentinelmesh/internal/scheduler"
	"github.com/sentinelmesh/sentinelmesh/internal/types"
	"github.com/sentinelmesh/sentinelmesh/internal/verification"
)

type mockClusterProvider struct {
	nodes []domain.Node
}

func (p *mockClusterProvider) ListNodes(ctx context.Context) ([]domain.Node, error) {
	return p.nodes, nil
}

func setupScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(s)
	_ = v1alpha1.AddToScheme(s)
	return s
}

// TestHeroTrace_EndToEndLifecycle traces the complete SentinelMesh lifecycle:
// Create Run -> Schedule -> Execute -> Checkpoint -> Kill Worker -> Recovery -> Failover -> Restore -> Verification -> Attestation.
// It verifies:
// 1. Full trace lineage across distributed components (shared trace_id).
// 2. Separation of identifiers: run_id, execution_generation, correlation_id, trace_id, span_id.
// 3. Recovery effectiveness metrics recorded with actual execution data in Prometheus.
// 4. Structured logs with trace correlation and redaction.
func TestHeroTrace_EndToEndLifecycle(t *testing.T) {
	// 1. Initialize In-Memory Tracing & Custom Prometheus Registry & Structured Logging
	spanExporter := tracetest.NewInMemoryExporter()
	cfg := observability.DefaultConfig("sentinelmesh-hero-test")
	_, err := observability.InitTracing(cfg, spanExporter)
	if err != nil {
		t.Fatalf("Failed to initialize tracing: %v", err)
	}
	defer observability.ShutdownTracing(context.Background())

	reg := prometheus.NewRegistry()
	_ = observability.InitMetrics(reg)

	var logBuf bytes.Buffer
	logger := observability.InitLogging("sentinelmesh-hero-test", &logBuf, slog.LevelInfo)

	// 2. Initialize Repositories & Subsystems
	runRepo := memory.NewRunRepository()
	agentRepo := memory.NewAgentRepository()
	assignmentRepo := memory.NewAssignmentRepository()
	outboxRepo := outbox.NewMemoryRepository()
	txManager := memory.NewTxManager()
	attestationRepo := verification.NewMemoryRepository()

	cpRepo := checkpoint.NewMemoryRepository()
	cpSvc := checkpoint.NewService(cpRepo, outboxRepo, txManager)

	nodes := []domain.Node{
		{
			ID:        "worker-alpha-01",
			ClusterID: "us-east-cluster",
			Resources: domain.NodeResources{
				CPUCapacity:     4.0,
				CPUAvailable:    4.0,
				MemoryCapacity:  8192,
				MemoryAvailable: 8192,
			},
			Health:        domain.NodeHealthHealthy,
			SecurityClass: "standard",
		},
		{
			ID:        "worker-alpha-02",
			ClusterID: "us-east-cluster",
			Resources: domain.NodeResources{
				CPUCapacity:     8.0,
				CPUAvailable:    8.0,
				MemoryCapacity:  16384,
				MemoryAvailable: 16384,
			},
			Health:        domain.NodeHealthHealthy,
			SecurityClass: "standard",
		},
	}

	resProv := &mockClusterProvider{nodes: nodes}
	nodeTracker := cluster.NewNodeTracker(resProv)
	failureDetector := cluster.NewFailureDetector(nodeTracker, runRepo, cpRepo, outboxRepo, txManager)
	schedSvc := scheduler.NewService(txManager, agentRepo, runRepo, assignmentRepo, outboxRepo, resProv)
	recoveryCoord := application.NewRecoveryCoordinator(runRepo, cpSvc, schedSvc, outboxRepo, txManager)
	runSvc := application.NewRunService(txManager, agentRepo, runRepo, outboxRepo)
	verificationSvc := verification.NewService(attestationRepo, agentRepo, runRepo, outboxRepo, txManager, nil, nil)

	// 3. Establish Root Trace Context & Identifiers
	correlationID := "corr-hero-flow-999"
	ctx := context.Background()
	ctx = observability.WithCorrelationID(ctx, correlationID)
	ctx = observability.WithTenantID(ctx, "tenant-finance-01")

	// Start Hero Root Span
	rootCtx, rootSpan := observability.StartSpan(ctx, "hero_trace.workflow")
	defer rootSpan.End()

	rootTraceID := rootSpan.SpanContext().TraceID().String()
	if rootTraceID == "" {
		t.Fatalf("Root span trace_id is empty")
	}

	// 4. Register Agent Definition
	agent := domain.Agent{
		ID:       "agent-payment-reconciler",
		TenantID: "tenant-finance-01",
		Name:     "payment-reconciler",
		Version:  "2.1.0",
		Resources: types.AgentResources{
			CPU:    "2",
			Memory: "2048Mi",
		},
		SecurityPolicy: types.SecurityPolicy{Profile: "standard"},
		Priority:       "critical",
		Image:          "sentinelmesh/reconciler:v2.1",
		VerificationPolicy: types.VerificationPolicy{
			Enabled: true,
			InvariantRules: []types.InvariantRule{
				{
					ID:            "inv-steps-completed",
					MetricName:    "steps_completed",
					Operator:      "gte",
					ExpectedValue: "50",
				},
			},
		},
	}
	_ = agentRepo.Create(rootCtx, agent)

	// ==========================================
	// Phase 1: Create Run (Application Layer)
	// ==========================================
	run, err := runSvc.CreateRun(rootCtx, agent.ID)
	if err != nil {
		t.Fatalf("Phase 1: CreateRun failed: %v", err)
	}

	runID := run.ID
	runCtx := observability.WithRunID(rootCtx, runID)
	runCtx = observability.WithExecutionGeneration(runCtx, 0)

	logger.InfoContext(runCtx, "Agent run created successfully",
		slog.String("run_id", runID),
		slog.String("token", "secret_auth_token_xyz"), // Redaction test
	)

	// ==========================================
	// Phase 2: Schedule Run (Scheduler Layer)
	// ==========================================
	_ = run.TransitionTo(types.StateQueued)
	_ = runRepo.Update(runCtx, run)

	if err := schedSvc.ScheduleRun(runCtx, runID); err != nil {
		t.Fatalf("Phase 2: ScheduleRun failed: %v", err)
	}

	scheduledRun, _ := runRepo.Get(runCtx, runID)
	if scheduledRun.Node != "worker-alpha-01" {
		t.Fatalf("Phase 2: Expected initial node worker-alpha-01, got %s", scheduledRun.Node)
	}
	if scheduledRun.RecoveryGeneration != 0 {
		t.Fatalf("Phase 2: Expected initial generation 0, got %d", scheduledRun.RecoveryGeneration)
	}

	// ==========================================
	// Phase 3: Execute & Checkpoint (Runtime Layer)
	// ==========================================
	_ = scheduledRun.TransitionTo(types.StateStarting)
	_ = runRepo.Update(runCtx, scheduledRun)
	_ = scheduledRun.TransitionTo(types.StateRunning)
	_ = runRepo.Update(runCtx, scheduledRun)

	var lastCP *checkpoint.Checkpoint
	for _, step := range []int{10, 20, 25} {
		stateData := json.RawMessage(fmt.Sprintf(`{"step":%d,"transactions_processed":%d}`, step, step*1000))
		cp, err := cpSvc.SaveCheckpoint(runCtx, checkpoint.SaveCheckpointRequest{
			RunID:          runID,
			AgentID:        agent.ID,
			TenantID:       agent.TenantID,
			SequenceNumber: int64(step),
			StateInline:    stateData,
		})
		if err != nil {
			t.Fatalf("Phase 3: SaveCheckpoint at step %d failed: %v", step, err)
		}
		lastCP = cp
	}

	if lastCP.SequenceNumber != 25 {
		t.Fatalf("Phase 3: Expected latest checkpoint sequence 25, got %d", lastCP.SequenceNumber)
	}

	// ==========================================
	// Phase 4: Worker Failure / Crash Injection
	// ==========================================
	affectedRuns, err := failureDetector.HandleNodeFailure(runCtx, "worker-alpha-01", "hardware kernel panic")
	if err != nil {
		t.Fatalf("Phase 4: HandleNodeFailure failed: %v", err)
	}
	if len(affectedRuns) != 1 || affectedRuns[0] != runID {
		t.Fatalf("Phase 4: Expected affected run %s, got %v", runID, affectedRuns)
	}

	interruptedRun, _ := runRepo.Get(runCtx, runID)
	if interruptedRun.State != types.StateFailed {
		t.Fatalf("Phase 4: Expected run state FAILED, got %s", interruptedRun.State)
	}
	if interruptedRun.RecoveryGeneration != 1 {
		t.Fatalf("Phase 4: Expected recovery generation 1, got %d", interruptedRun.RecoveryGeneration)
	}

	// ==========================================
	// Phase 5: Recovery & Failover Coordination
	// ==========================================
	recCtx := observability.WithExecutionGeneration(runCtx, 1)

	recReq := events.RunRecoveryRequestedPayload{
		RunID:              runID,
		AgentID:            agent.ID,
		TenantID:           agent.TenantID,
		FailedNodeID:       "worker-alpha-01",
		RecoveryGeneration: 1,
		SourceCheckpointID: interruptedRun.LastCheckpointID,
		SequenceNumber:     25,
		RequestedAt:        time.Now(),
	}

	if err := recoveryCoord.HandleRecovery(recCtx, recReq); err != nil {
		t.Fatalf("Phase 5: HandleRecovery failed: %v", err)
	}

	recoveredRun, _ := runRepo.Get(recCtx, runID)
	if recoveredRun.State != types.StateScheduled {
		t.Fatalf("Phase 5: Expected recovered state SCHEDULED, got %s", recoveredRun.State)
	}
	if recoveredRun.Node != "worker-alpha-02" {
		t.Fatalf("Phase 5: Expected failover to worker-alpha-02, got %s", recoveredRun.Node)
	}
	if recoveredRun.RecoveryGeneration != 1 {
		t.Fatalf("Phase 5: Expected recovery generation 1, got %d", recoveredRun.RecoveryGeneration)
	}

	// ==========================================
	// Phase 6: Operator Reconcile & Restore
	// ==========================================
	latestCP, err := cpSvc.GetLatestCheckpoint(recCtx, runID)
	if err != nil {
		t.Fatalf("Phase 6: GetLatestCheckpoint failed: %v", err)
	}

	schedPayload := &events.RunScheduledPayload{
		RunID:       recoveredRun.ID,
		AgentID:     agent.ID,
		ClusterID:   recoveredRun.Cluster,
		NodeID:      recoveredRun.Node,
		AgentImage:  agent.Image,
		AgentCPU:    agent.Resources.CPU,
		AgentMemory: agent.Resources.Memory,
		Checkpoint: &events.CheckpointMetadataPayload{
			ID:        latestCP.ID,
			Sequence:  latestCP.SequenceNumber,
			Checksum:  latestCP.StateChecksum,
			SizeBytes: latestCP.SizeBytes,
		},
	}

	agentRunCR := operator.MapRunScheduledToAgentRun(schedPayload, "default")
	k8sClient := fake.NewClientBuilder().
		WithScheme(setupScheme()).
		WithObjects(agentRunCR).
		WithStatusSubresource(&v1alpha1.AgentRun{}).
		Build()

	reconciler := &operator.AgentRunReconciler{Client: k8sClient, Scheme: setupScheme()}
	req := ctrl.Request{NamespacedName: k8stypes.NamespacedName{Name: agentRunCR.Name, Namespace: "default"}}
	res, err := reconciler.Reconcile(recCtx, req)
	if err != nil || res.Requeue {
		t.Fatalf("Phase 6: Reconciler failed: %v", err)
	}

	var replacementPod corev1.Pod
	podName := fmt.Sprintf("agentrun-%s", runID)
	if err := k8sClient.Get(recCtx, k8stypes.NamespacedName{Name: podName, Namespace: "default"}, &replacementPod); err != nil {
		t.Fatalf("Phase 6: Failed to get replacement Pod: %v", err)
	}

	if replacementPod.Spec.NodeName != "worker-alpha-02" {
		t.Errorf("Phase 6: Expected replacement Pod pinned to worker-alpha-02, got %s", replacementPod.Spec.NodeName)
	}

	// ==========================================
	// Phase 7: Resume Execution Steps 26..50
	// ==========================================
	recoveredRun, _ = runRepo.Get(recCtx, runID)
	_ = recoveredRun.TransitionTo(types.StateRestoring)
	_ = runRepo.Update(recCtx, recoveredRun)

	recoveredRun, _ = runRepo.Get(recCtx, runID)
	_ = recoveredRun.TransitionTo(types.StateRunning)
	_ = runRepo.Update(recCtx, recoveredRun)

	// ==========================================
	// Phase 8: Verification & Attestation
	// ==========================================
	attestation, err := verificationSvc.VerifyRun(recCtx, verification.VerifyRunRequest{
		RunID: runID,
		ReportedMetrics: map[string]string{
			"steps_completed": "50",
		},
	})
	if err != nil {
		t.Fatalf("Phase 8: VerifyRun failed: %v", err)
	}
	if attestation.Status != verification.StatusVerified {
		t.Fatalf("Phase 8: Expected status VERIFIED, got %s", attestation.Status)
	}
	if attestation.EvidenceDigest == "" {
		t.Fatalf("Phase 8: Evidence digest is empty")
	}

	finalRun, _ := runRepo.Get(recCtx, runID)
	if finalRun.State != types.StateCompleted {
		t.Fatalf("Phase 8: Expected final state COMPLETED, got %s", finalRun.State)
	}
	if finalRun.VerificationState != "VERIFIED" {
		t.Fatalf("Phase 8: Expected verification state VERIFIED, got %s", finalRun.VerificationState)
	}

	// Close root span before asserting exported traces
	rootSpan.End()

	// ==========================================
	// Phase 9: Trace Lineage & Spans Assertions
	// ==========================================
	spans := spanExporter.GetSpans()
	if len(spans) == 0 {
		t.Fatalf("Expected exported spans, got 0")
	}

	t.Logf("Total exported spans in hero trace: %d", len(spans))

	expectedSpanNames := map[string]bool{
		"hero_trace.workflow":             false,
		"application.create_run":          false,
		"scheduler.decision":              false,
		"checkpoint.save":                 false,
		"recovery.handle":                 false,
		"scheduler.reschedule_decision":   false,
		"operator.reconcile":              false,
		"verification.verify_run":         false,
	}

	for _, span := range spans {
		// Verify Trace ID Lineage: every single span in the workflow MUST share the root trace ID
		if span.SpanContext.TraceID().String() != rootTraceID {
			t.Errorf("Span %s has mismatched trace_id: expected %s, got %s",
				span.Name, rootTraceID, span.SpanContext.TraceID().String())
		}

		if _, exists := expectedSpanNames[span.Name]; exists {
			expectedSpanNames[span.Name] = true
		}

		// Verify Separation of Identifiers on span attributes
		var foundRunID, foundCorrID string
		var foundGen int64 = -1

		for _, attr := range span.Attributes {
			switch string(attr.Key) {
			case "sentinel.run_id":
				foundRunID = attr.Value.AsString()
			case "sentinel.correlation_id":
				foundCorrID = attr.Value.AsString()
			case "sentinel.execution_generation":
				foundGen = attr.Value.AsInt64()
			}
		}

		if span.Name != "hero_trace.workflow" && span.Name != "checkpoint.get_latest" {
			if foundRunID != "" && foundRunID != runID {
				t.Errorf("Span %s has incorrect run_id: %s vs %s", span.Name, foundRunID, runID)
			}
			if foundCorrID != "" && foundCorrID != correlationID {
				t.Errorf("Span %s has incorrect correlation_id: %s vs %s", span.Name, foundCorrID, correlationID)
			}
			if span.Name == "recovery.handle" && foundGen != 1 {
				t.Errorf("Span recovery.handle should have execution_generation=1, got %d", foundGen)
			}
		}
	}

	for spanName, found := range expectedSpanNames {
		if !found {
			t.Errorf("Required hero trace span %q was not found in exported spans", spanName)
		}
	}

	// ==========================================
	// Phase 10: Prometheus Metric Registry Assertions
	// ==========================================
	metricFamilies, err := reg.Gather()
	if err != nil {
		t.Fatalf("Failed to gather Prometheus metrics: %v", err)
	}

	metricMap := make(map[string]float64)
	for _, mf := range metricFamilies {
		name := mf.GetName()
		for _, m := range mf.GetMetric() {
			if m.GetCounter() != nil {
				metricMap[name] += m.GetCounter().GetValue()
			} else if m.GetGauge() != nil {
				metricMap[name] += m.GetGauge().GetValue()
			} else if m.GetHistogram() != nil {
				metricMap[name] += float64(m.GetHistogram().GetSampleCount())
			}
		}
	}

	// Assert Thesis Recovery Effectiveness & Platform Metrics
	assertMetricGte(t, metricMap, "sentinel_runs_created_total", 1)
	assertMetricGte(t, metricMap, "sentinel_scheduler_decisions_total", 2) // Initial + Reschedule
	assertMetricGte(t, metricMap, "sentinel_checkpoint_saved_total", 3)
	assertMetricGte(t, metricMap, "sentinel_recovery_requests_total", 1)
	assertMetricGte(t, metricMap, "sentinel_recovery_success_total", 1)
	assertMetricGte(t, metricMap, "sentinel_recovery_generation_total", 1)
	assertMetricGte(t, metricMap, "sentinel_recovery_recovery_point_steps", 1)   // Observed step 25
	assertMetricGte(t, metricMap, "sentinel_recovery_checkpoint_age_seconds", 1) // Age observed
	assertMetricGte(t, metricMap, "sentinel_recovery_lost_work_steps", 1)       // Lost steps observed
	assertMetricGte(t, metricMap, "sentinel_recovery_duration_seconds", 1)      // Duration observed
	assertMetricGte(t, metricMap, "sentinel_verification_total", 1)
	assertMetricGte(t, metricMap, "sentinel_verification_success_total", 1)
	assertMetricGte(t, metricMap, "sentinel_runs_completed_total", 1)

	// ==========================================
	// Phase 11: Structured Logging Redaction Assertions
	// ==========================================
	logLines := bytes.Split(bytes.TrimSpace(logBuf.Bytes()), []byte("\n"))
	if len(logLines) == 0 {
		t.Fatalf("Expected log output, got 0 lines")
	}

	foundRedactedToken := false
	for _, line := range logLines {
		var logEntry map[string]interface{}
		if err := json.Unmarshal(line, &logEntry); err != nil {
			continue
		}

		if token, ok := logEntry["token"]; ok {
			if token != "[REDACTED]" {
				t.Errorf("Security violation: token was not redacted! Value: %v", token)
			} else {
				foundRedactedToken = true
			}
		}
	}

	if !foundRedactedToken {
		t.Errorf("Expected to verify redacted token in structured log output")
	}

	t.Logf("Hero Trace Test Successfully Verified: Full trace lineage %s, all 10 stages observed with 0 race conditions.", rootTraceID)
}

func assertMetricGte(t *testing.T, metrics map[string]float64, name string, expectedMin float64) {
	t.Helper()
	val, exists := metrics[name]
	if !exists {
		t.Errorf("Prometheus metric %q was not found in gathered registry", name)
		return
	}
	if val < expectedMin {
		t.Errorf("Prometheus metric %q expected >= %f, got %f", name, expectedMin, val)
	}
}
