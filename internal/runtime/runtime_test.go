package runtime

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"

	"github.com/sentinelmesh/sentinelmesh/internal/types"
)

func TestProcessRuntime_Lifecycle_Success(t *testing.T) {
	rt := NewProcessRuntime()
	ctx := context.Background()

	req := ExecutionRequest{
		RunID:    "run-success-1",
		AgentID:  "agent-1",
		TenantID: "tenant-1",
		Command:  "sh",
		Args:     []string{"-c", "echo 'starting workload'; echo 'step 1 done'; echo 'completed'"},
	}

	handle, err := rt.Start(ctx, req)
	if err != nil {
		t.Fatalf("failed to start process: %v", err)
	}

	if handle.ProcessID <= 0 {
		t.Errorf("invalid ProcessID: %d", handle.ProcessID)
	}

	// Wait for process to complete
	var finalStatus *ExecutionStatus
	for i := 0; i < 20; i++ {
		st, err := rt.Status(ctx, "run-success-1")
		if err != nil {
			t.Fatalf("failed to get status: %v", err)
		}
		if st.State.IsTerminal() {
			finalStatus = st
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	if finalStatus == nil {
		t.Fatal("process did not complete in time")
	}

	if finalStatus.State != types.StateCompleted {
		t.Errorf("expected state COMPLETED, got %s", finalStatus.State)
	}
	if finalStatus.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", finalStatus.ExitCode)
	}
	if finalStatus.FinishedAt == nil {
		t.Errorf("expected FinishedAt to be set")
	}

	// Verify log stream captured all output
	logStream, err := rt.Logs(ctx, "run-success-1", LogOptions{Follow: false})
	if err != nil {
		t.Fatalf("failed to fetch logs: %v", err)
	}
	defer logStream.Close()

	var logs bytes.Buffer
	_, _ = io.Copy(&logs, logStream)

	logStr := logs.String()
	if !bytes.Contains(logs.Bytes(), []byte("starting workload")) ||
		!bytes.Contains(logs.Bytes(), []byte("completed")) {
		t.Errorf("log output missing expected content, got:\n%s", logStr)
	}

	// Clean up
	_ = rt.Delete(ctx, "run-success-1")
}

func TestProcessRuntime_Lifecycle_FailureExitCode(t *testing.T) {
	rt := NewProcessRuntime()
	ctx := context.Background()

	req := ExecutionRequest{
		RunID:    "run-fail-1",
		AgentID:  "agent-1",
		TenantID: "tenant-1",
		Command:  "sh",
		Args:     []string{"-c", "echo 'failing soon' >&2; exit 42"},
	}

	_, err := rt.Start(ctx, req)
	if err != nil {
		t.Fatalf("failed to start process: %v", err)
	}

	var finalStatus *ExecutionStatus
	for i := 0; i < 20; i++ {
		st, err := rt.Status(ctx, "run-fail-1")
		if err == nil && st.IsFinished() {
			finalStatus = st
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	if finalStatus == nil {
		t.Fatal("process did not complete in time")
	}

	if finalStatus.State != types.StateFailed {
		t.Errorf("expected state FAILED, got %s", finalStatus.State)
	}
	if finalStatus.ExitCode != 42 {
		t.Errorf("expected exit code 42, got %d", finalStatus.ExitCode)
	}
}

func TestProcessRuntime_StopGracefully(t *testing.T) {
	rt := NewProcessRuntime()
	ctx := context.Background()

	req := ExecutionRequest{
		RunID:    "run-stop-1",
		AgentID:  "agent-1",
		TenantID: "tenant-1",
		Command:  "sleep",
		Args:     []string{"10"},
	}

	_, err := rt.Start(ctx, req)
	if err != nil {
		t.Fatalf("failed to start process: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	err = rt.Stop(ctx, "run-stop-1", 500*time.Millisecond)
	if err != nil {
		t.Fatalf("stop failed: %v", err)
	}

	st, err := rt.Status(ctx, "run-stop-1")
	if err != nil {
		t.Fatalf("status failed: %v", err)
	}
	if !st.IsFinished() {
		t.Errorf("expected stopped process to have finished state, got %s", st.State)
	}
}

func TestProcessRuntime_PauseAndResume(t *testing.T) {
	rt := NewProcessRuntime()
	ctx := context.Background()

	req := ExecutionRequest{
		RunID:    "run-pause-1",
		AgentID:  "agent-1",
		TenantID: "tenant-1",
		Command:  "sleep",
		Args:     []string{"10"},
	}

	_, err := rt.Start(ctx, req)
	if err != nil {
		t.Fatalf("failed to start process: %v", err)
	}
	defer rt.Delete(ctx, "run-pause-1")

	time.Sleep(50 * time.Millisecond)

	// Pause
	if err := rt.Pause(ctx, "run-pause-1"); err != nil {
		t.Fatalf("pause failed: %v", err)
	}

	st, _ := rt.Status(ctx, "run-pause-1")
	if st.State != types.StatePaused {
		t.Errorf("expected state PAUSED, got %s", st.State)
	}

	// Resume
	if err := rt.Resume(ctx, "run-pause-1"); err != nil {
		t.Fatalf("resume failed: %v", err)
	}

	st, _ = rt.Status(ctx, "run-pause-1")
	if st.State != types.StateRunning {
		t.Errorf("expected state RUNNING, got %s", st.State)
	}
}

func TestProcessRuntime_TimeoutEnforcement(t *testing.T) {
	rt := NewProcessRuntime()
	ctx := context.Background()

	req := ExecutionRequest{
		RunID:    "run-timeout-1",
		AgentID:  "agent-1",
		TenantID: "tenant-1",
		Command:  "sleep",
		Args:     []string{"10"},
		Timeout:  150 * time.Millisecond,
	}

	_, err := rt.Start(ctx, req)
	if err != nil {
		t.Fatalf("failed to start process: %v", err)
	}

	var finalStatus *ExecutionStatus
	for i := 0; i < 20; i++ {
		st, err := rt.Status(ctx, "run-timeout-1")
		if err == nil && st.IsFinished() {
			finalStatus = st
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	if finalStatus == nil {
		t.Fatal("process did not time out in time")
	}

	if finalStatus.State != types.StateFailed {
		t.Errorf("expected state FAILED on timeout, got %s", finalStatus.State)
	}
}
