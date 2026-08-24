package runtime

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/sentinelmesh/sentinelmesh/internal/types"
)

func TestSupervisor_StateTransitionDetection(t *testing.T) {
	rt := NewProcessRuntime()
	supervisor := NewSupervisor(rt, 50*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var mu sync.Mutex
	observedStates := make([]types.AgentState, 0)
	doneCh := make(chan struct{})

	supervisor.OnStateChange(func(st ExecutionStatus) {
		mu.Lock()
		observedStates = append(observedStates, st.State)
		if st.State.IsTerminal() {
			close(doneCh)
		}
		mu.Unlock()
	})

	supervisor.Start(ctx)
	defer supervisor.Stop()

	req := ExecutionRequest{
		RunID:    "run-sup-1",
		AgentID:  "agent-1",
		TenantID: "tenant-1",
		Command:  "sh",
		Args:     []string{"-c", "sleep 0.1; echo 'done'"},
	}

	_, err := rt.Start(ctx, req)
	if err != nil {
		t.Fatalf("failed to start process: %v", err)
	}

	supervisor.Register(req)

	select {
	case <-doneCh:
	case <-time.After(3 * time.Second):
		t.Fatal("supervisor did not observe terminal state in time")
	}

	mu.Lock()
	defer mu.Unlock()

	foundCompleted := false
	for _, s := range observedStates {
		if s == types.StateCompleted {
			foundCompleted = true
			break
		}
	}

	if !foundCompleted {
		t.Errorf("expected observed states to contain COMPLETED, got: %v", observedStates)
	}
}

func TestSupervisor_WatchdogTimeout(t *testing.T) {
	rt := NewProcessRuntime()
	supervisor := NewSupervisor(rt, 30*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	doneCh := make(chan struct{})
	supervisor.OnStateChange(func(st ExecutionStatus) {
		if st.IsFinished() {
			select {
			case <-doneCh:
			default:
				close(doneCh)
			}
		}
	})

	supervisor.Start(ctx)
	defer supervisor.Stop()

	req := ExecutionRequest{
		RunID:    "run-sup-timeout",
		AgentID:  "agent-1",
		TenantID: "tenant-1",
		Command:  "sleep",
		Args:     []string{"10"},
		Timeout:  100 * time.Millisecond,
	}

	_, err := rt.Start(ctx, req)
	if err != nil {
		t.Fatalf("failed to start process: %v", err)
	}

	supervisor.Register(req)

	select {
	case <-doneCh:
	case <-time.After(2 * time.Second):
		t.Fatal("supervisor watchdog did not terminate timed out run")
	}

	st, err := rt.Status(ctx, "run-sup-timeout")
	if err != nil {
		t.Fatalf("failed to get status: %v", err)
	}
	if !st.IsFinished() {
		t.Errorf("expected state to be finished, got %s", st.State)
	}
}
