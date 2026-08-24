package runtime

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/sentinelmesh/sentinelmesh/internal/types"
)

var (
	ErrRunNotFound      = errors.New("runtime: run not found")
	ErrRunAlreadyExists = errors.New("runtime: run already exists")
	ErrRunNotRunning    = errors.New("runtime: run is not running")
	ErrRunAlreadyPaused = errors.New("runtime: run is already paused")
	ErrRunNotPaused     = errors.New("runtime: run is not paused")
	ErrInvalidRequest   = errors.New("runtime: invalid execution request")
)

// ResourceLimits defines resource constraints for the runtime execution.
type ResourceLimits struct {
	CPULimit      string `json:"cpu_limit,omitempty"`
	MemoryLimitMB int64  `json:"memory_limit_mb,omitempty"`
	GPUCount      int    `json:"gpu_count,omitempty"`
}

// ExecutionRequest carries parameters necessary to launch an agent workload.
type ExecutionRequest struct {
	RunID          string            `json:"run_id"`
	AgentID        string            `json:"agent_id"`
	TenantID       string            `json:"tenant_id"`
	Command        string            `json:"command"`
	Args           []string          `json:"args,omitempty"`
	Env            map[string]string `json:"env,omitempty"`
	WorkingDir     string            `json:"working_dir,omitempty"`
	Timeout        time.Duration     `json:"timeout,omitempty"`
	ResourceLimits ResourceLimits    `json:"resource_limits,omitempty"`
}

// ExecutionHandle is returned upon starting an agent run.
type ExecutionHandle struct {
	RunID     string    `json:"run_id"`
	ProcessID int       `json:"process_id,omitempty"`
	PodName   string    `json:"pod_name,omitempty"`
	StartTime time.Time `json:"start_time"`
}

// ExecutionStatus captures the observed runtime state of an agent execution.
type ExecutionStatus struct {
	RunID        string           `json:"run_id"`
	State        types.AgentState `json:"state"`
	ExitCode     int              `json:"exit_code"`
	ErrorReason  string           `json:"error_reason,omitempty"`
	StartedAt    *time.Time       `json:"started_at,omitempty"`
	FinishedAt   *time.Time       `json:"finished_at,omitempty"`
	BytesRead    int64            `json:"bytes_read"`
	BytesWritten int64            `json:"bytes_written"`
}

// IsFinished returns true if the execution has concluded (Completed, Failed, or Cancelled).
func (s *ExecutionStatus) IsFinished() bool {
	return s.State == types.StateCompleted || s.State == types.StateFailed || s.State == types.StateCancelled
}

// LogOptions specifies filtering and streaming parameters when requesting logs.
type LogOptions struct {
	Follow     bool      `json:"follow"`
	TailLines  int       `json:"tail_lines"`
	Timestamps bool      `json:"timestamps"`
	Since      time.Time `json:"since,omitempty"`
}

// Runtime is the core abstraction for executing and managing agent workloads.
// Implementations exist for OS processes (local/testing), Containers, and Kubernetes Pods.
type Runtime interface {
	// Start launches the agent workload described in the request.
	Start(ctx context.Context, req ExecutionRequest) (*ExecutionHandle, error)

	// Stop terminates the running workload, honoring the grace period before forceful kill.
	Stop(ctx context.Context, runID string, gracePeriod time.Duration) error

	// Pause temporarily suspends execution of the running workload.
	Pause(ctx context.Context, runID string) error

	// Resume restores execution of a paused workload.
	Resume(ctx context.Context, runID string) error

	// Status queries the observed runtime state and exit code of the workload.
	Status(ctx context.Context, runID string) (*ExecutionStatus, error)

	// Logs returns a streaming reader for the combined stdout/stderr output.
	Logs(ctx context.Context, runID string, opts LogOptions) (io.ReadCloser, error)

	// Delete releases all runtime resources (buffers, processes, state) associated with the run.
	Delete(ctx context.Context, runID string) error
}
