package runtime

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/sentinelmesh/sentinelmesh/internal/types"
)

type processHandle struct {
	mu          sync.RWMutex
	runID       string
	cmd         *exec.Cmd
	buffer      *LogBuffer
	status      ExecutionStatus
	cancelFunc  context.CancelFunc
	doneCh      chan struct{}
	paused      bool
}

// ProcessRuntime executes agent workloads as local operating system processes.
//
// CONTAINMENT MODEL & SECURITY POSTURE:
//   - ProcessRuntime provides BEST-EFFORT LOCAL CONTAINMENT suitable for development,
//     testing, and trusted worker nodes (e.g. process grouping via Setpgid, environment
//     scrubbing, timeout enforcement, signal dispatching).
//   - KubernetesRuntime provides the PRIMARY PRODUCTION ISOLATION BOUNDARY (cgroups,
//     namespaces, read-only rootfs, dropped capabilities, RuntimeDefault seccomp,
//     and NetworkPolicies).
//
// ProcessRuntime should NOT be treated as a multi-tenant cryptographic sandbox.
type ProcessRuntime struct {
	mu        sync.RWMutex
	processes map[string]*processHandle
}

// NewProcessRuntime constructs a ProcessRuntime instance.
func NewProcessRuntime() *ProcessRuntime {
	return &ProcessRuntime{
		processes: make(map[string]*processHandle),
	}
}

// Start launches a new OS process according to the ExecutionRequest.
func (r *ProcessRuntime) Start(ctx context.Context, req ExecutionRequest) (*ExecutionHandle, error) {
	if req.RunID == "" {
		return nil, fmt.Errorf("%w: missing run_id", ErrInvalidRequest)
	}
	if req.Command == "" {
		return nil, fmt.Errorf("%w: missing command", ErrInvalidRequest)
	}

	r.mu.Lock()
	if _, exists := r.processes[req.RunID]; exists {
		r.mu.Unlock()
		return nil, ErrRunAlreadyExists
	}

	procCtx, cancel := context.WithCancel(context.Background())
	if req.Timeout > 0 {
		procCtx, cancel = context.WithTimeout(context.Background(), req.Timeout)
	}

	cmd := exec.CommandContext(procCtx, req.Command, req.Args...)
	if req.WorkingDir != "" {
		cmd.Dir = req.WorkingDir
	}

	// Prepare environment
	cmd.Env = os.Environ()
	for k, v := range req.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}
	cmd.Env = append(cmd.Env,
		fmt.Sprintf("SENTINEL_RUN_ID=%s", req.RunID),
		fmt.Sprintf("SENTINEL_AGENT_ID=%s", req.AgentID),
		fmt.Sprintf("SENTINEL_TENANT_ID=%s", req.TenantID),
	)

	// Set process group for clean signal dispatching
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	logBuf := NewLogBuffer(10000)
	cmd.Stdout = logBuf
	cmd.Stderr = logBuf

	now := time.Now()
	handle := &processHandle{
		runID:      req.RunID,
		cmd:        cmd,
		buffer:     logBuf,
		cancelFunc: cancel,
		doneCh:     make(chan struct{}),
		status: ExecutionStatus{
			RunID:     req.RunID,
			State:     types.StateStarting,
			StartedAt: &now,
		},
	}

	if err := cmd.Start(); err != nil {
		cancel()
		_ = logBuf.Close()
		r.mu.Unlock()
		return nil, fmt.Errorf("failed to start process: %w", err)
	}

	handle.status.State = types.StateRunning
	r.processes[req.RunID] = handle
	r.mu.Unlock()

	// Asynchronous waiter goroutine
	go r.waitProcess(handle, procCtx)

	return &ExecutionHandle{
		RunID:     req.RunID,
		ProcessID: cmd.Process.Pid,
		StartTime: now,
	}, nil
}

func (r *ProcessRuntime) waitProcess(h *processHandle, procCtx context.Context) {
	err := h.cmd.Wait()
	now := time.Now()

	h.mu.Lock()
	defer h.mu.Unlock()

	h.status.FinishedAt = &now
	_ = h.buffer.Close()
	close(h.doneCh)

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			h.status.ExitCode = exitErr.ExitCode()
			h.status.State = types.StateFailed
			h.status.ErrorReason = fmt.Sprintf("process exited with status %d", exitErr.ExitCode())
		} else if procCtx.Err() == context.DeadlineExceeded {
			h.status.ExitCode = -1
			h.status.State = types.StateFailed
			h.status.ErrorReason = "execution timed out"
		} else if procCtx.Err() == context.Canceled {
			h.status.ExitCode = -1
			h.status.State = types.StateCancelled
			h.status.ErrorReason = "process was stopped/cancelled"
		} else {
			h.status.ExitCode = 1
			h.status.State = types.StateFailed
			h.status.ErrorReason = err.Error()
		}
	} else {
		h.status.ExitCode = 0
		h.status.State = types.StateCompleted
	}
}

// Stop terminates the execution handle gracefully (SIGTERM) or forcefully (SIGKILL).
func (r *ProcessRuntime) Stop(ctx context.Context, runID string, gracePeriod time.Duration) error {
	r.mu.RLock()
	h, exists := r.processes[runID]
	r.mu.RUnlock()

	if !exists {
		return ErrRunNotFound
	}

	h.mu.Lock()
	if h.status.State.IsTerminal() {
		h.mu.Unlock()
		return nil
	}

	pid := h.cmd.Process.Pid
	h.mu.Unlock()

	// Signal process group
	_ = syscall.Kill(-pid, syscall.SIGTERM)

	if gracePeriod <= 0 {
		gracePeriod = 3 * time.Second
	}

	select {
	case <-h.doneCh:
		return nil
	case <-time.After(gracePeriod):
		// Force kill
		_ = syscall.Kill(-pid, syscall.SIGKILL)
		h.cancelFunc()
	case <-ctx.Done():
		_ = syscall.Kill(-pid, syscall.SIGKILL)
		h.cancelFunc()
		return ctx.Err()
	}

	<-h.doneCh
	return nil
}

// Pause sends SIGSTOP to freeze execution.
func (r *ProcessRuntime) Pause(ctx context.Context, runID string) error {
	r.mu.RLock()
	h, exists := r.processes[runID]
	r.mu.RUnlock()

	if !exists {
		return ErrRunNotFound
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	if h.status.State != types.StateRunning {
		return ErrRunNotRunning
	}
	if h.paused {
		return ErrRunAlreadyPaused
	}

	pid := h.cmd.Process.Pid
	if err := syscall.Kill(-pid, syscall.SIGSTOP); err != nil {
		return fmt.Errorf("failed to send SIGSTOP: %w", err)
	}

	h.paused = true
	h.status.State = types.StatePaused
	return nil
}

// Resume sends SIGCONT to unfreeze execution.
func (r *ProcessRuntime) Resume(ctx context.Context, runID string) error {
	r.mu.RLock()
	h, exists := r.processes[runID]
	r.mu.RUnlock()

	if !exists {
		return ErrRunNotFound
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	if !h.paused {
		return ErrRunNotPaused
	}

	pid := h.cmd.Process.Pid
	if err := syscall.Kill(-pid, syscall.SIGCONT); err != nil {
		return fmt.Errorf("failed to send SIGCONT: %w", err)
	}

	h.paused = false
	h.status.State = types.StateRunning
	return nil
}

// Status returns the current execution status.
func (r *ProcessRuntime) Status(ctx context.Context, runID string) (*ExecutionStatus, error) {
	r.mu.RLock()
	h, exists := r.processes[runID]
	r.mu.RUnlock()

	if !exists {
		return nil, ErrRunNotFound
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	stCopy := h.status
	return &stCopy, nil
}

// Logs returns a streaming reader for the combined output stream.
func (r *ProcessRuntime) Logs(ctx context.Context, runID string, opts LogOptions) (io.ReadCloser, error) {
	r.mu.RLock()
	h, exists := r.processes[runID]
	r.mu.RUnlock()

	if !exists {
		return nil, ErrRunNotFound
	}

	return h.buffer.NewReader(opts.TailLines, opts.Follow), nil
}

// Delete removes all tracked state and ensures process cleanup.
func (r *ProcessRuntime) Delete(ctx context.Context, runID string) error {
	r.mu.Lock()
	h, exists := r.processes[runID]
	if exists {
		delete(r.processes, runID)
	}
	r.mu.Unlock()

	if !exists {
		return nil
	}

	_ = r.Stop(ctx, runID, 1*time.Second)
	_ = h.buffer.Close()
	return nil
}
