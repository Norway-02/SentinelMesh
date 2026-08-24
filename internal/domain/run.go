package domain

import (
	"fmt"
	"time"

	"github.com/sentinelmesh/sentinelmesh/internal/types"
)

// AgentRun represents a specific execution instance of an Agent.
type AgentRun struct {
	ID                string
	AgentID           string
	TenantID          string
	State             types.AgentState
	Attempt           int
	Node              string
	Cluster           string
	StartedAt         *time.Time
	FinishedAt                 *time.Time
	LastCheckpointID           string
	RecoveredFromCheckpointID  string
	RecoveryGeneration         int
	FencingToken               string
	RetryCount                 int
	FailureReason              string
	VerificationState          string
	AttestationID              string
	Version                    int64 // For optimistic concurrency control
	CreatedAt                  time.Time
	UpdatedAt                  time.Time
}

// Validate checks basic properties of AgentRun construction.
func (r *AgentRun) Validate() error {
	if err := ValidateIdentifier(r.ID); err != nil {
		return fmt.Errorf("%w: invalid run ID: %v", ErrInvalidAgentRun, err)
	}
	if err := ValidateIdentifier(r.AgentID); err != nil {
		return fmt.Errorf("%w: invalid agent ID: %v", ErrInvalidAgentRun, err)
	}
	if r.TenantID == "" {
		return fmt.Errorf("%w: missing tenant ID", ErrInvalidAgentRun)
	}
	if r.State == "" {
		return fmt.Errorf("%w: state cannot be empty", ErrInvalidAgentRun)
	}
	if r.RetryCount < 0 {
		return fmt.Errorf("%w: retry count cannot be negative", ErrInvalidAgentRun)
	}
	return nil
}

// TransitionTo changes the state of the run if the transition is valid.
func (r *AgentRun) TransitionTo(next types.AgentState) error {
	if err := ValidateTransition(r.State, next); err != nil {
		return err
	}
	
	now := time.Now()
	r.UpdatedAt = now
	r.State = next

	if next == types.StateStarting || next == types.StateRunning {
		if r.StartedAt == nil {
			r.StartedAt = &now
		}
	}

	if next == types.StateCompleted || next == types.StateCancelled || next == types.StateFailed {
		r.FinishedAt = &now
	}

	return nil
}

// CanRetry determines if the run can attempt a retry.
func (r *AgentRun) CanRetry(maxRetries int) bool {
	if maxRetries < 0 {
		return false
	}
	return r.RetryCount < maxRetries
}
