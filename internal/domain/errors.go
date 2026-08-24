package domain

import "errors"

// Sentinel errors representing domain-level failures.
var (
	ErrInvalidStateTransition = errors.New("invalid state transition")
	ErrInvalidAgent           = errors.New("invalid agent configuration")
	ErrInvalidAgentRun        = errors.New("invalid agent run configuration")
	ErrInvalidCheckpoint      = errors.New("invalid checkpoint configuration")
	ErrInvalidPolicy          = errors.New("invalid policy configuration")
	ErrRetryLimitExceeded     = errors.New("retry limit exceeded")
)
