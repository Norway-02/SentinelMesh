package domain

import (
	"fmt"
	"time"
)

// Checkpoint represents metadata about a saved state of an AgentRun.
type Checkpoint struct {
	ID          string
	RunID       string
	Version     int
	State       string
	ArtifactURI string
	Checksum    string
	CreatedAt   time.Time
}

// Validate checks basic properties of Checkpoint construction.
func (c *Checkpoint) Validate() error {
	if err := ValidateIdentifier(c.ID); err != nil {
		return fmt.Errorf("%w: invalid checkpoint ID: %v", ErrInvalidCheckpoint, err)
	}
	if err := ValidateIdentifier(c.RunID); err != nil {
		return fmt.Errorf("%w: invalid run ID: %v", ErrInvalidCheckpoint, err)
	}
	if c.Version < 1 {
		return fmt.Errorf("%w: version must be >= 1", ErrInvalidCheckpoint)
	}
	if c.ArtifactURI == "" {
		return fmt.Errorf("%w: artifact URI cannot be empty", ErrInvalidCheckpoint)
	}
	// Checksum validation would depend on specific hashing algorithm, 
	// but minimally shouldn't be empty for a complete artifact.
	if c.Checksum == "" {
		return fmt.Errorf("%w: checksum cannot be empty", ErrInvalidCheckpoint)
	}

	return nil
}
